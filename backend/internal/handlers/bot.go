package handlers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/ilyas/vpn-service/backend/internal/auth"
	"github.com/ilyas/vpn-service/backend/internal/store"
)

type botEnsureRequest struct {
	TelegramID int64  `json:"telegram_id"`
	Username   string `json:"username"`
	FirstName  string `json:"first_name"`
}

type botUserResponse struct {
	TelegramID      int64  `json:"telegram_id"`
	Username        string `json:"username"`
	Provisioned     bool   `json:"provisioned"`
	SubscriptionURL string `json:"subscription_url,omitempty"`
	VLESS           string `json:"vless,omitempty"`
	SingBox         string `json:"singbox,omitempty"`
	ExpiresAt       int64  `json:"expires_at,omitempty"`
}

type botExpiringResponse struct {
	TelegramID int64  `json:"telegram_id"`
	Username   string `json:"username"`
	ExpiresAt  int64  `json:"expires_at"`
}

// ensureBotUser finds (or creates) the VPN user backing a Telegram account and
// returns everything the bot needs to deliver the key: subscription URL, VLESS
// config and Sing-box config.
func (h *Handler) ensureBotUser(c *gin.Context) {
	var body botEnsureRequest
	if err := c.ShouldBindJSON(&body); err != nil || body.TelegramID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "telegram_id is required"})
		return
	}

	ctx := c.Request.Context()

	user, err := h.store.GetUserByTelegramID(ctx, body.TelegramID)
	if errors.Is(err, store.ErrNotFound) {
		// Fall back to the legacy WebApp-created account (tg_<id>) and link it.
		user, err = h.store.GetUserByUsername(ctx, fmt.Sprintf("tg_%d", body.TelegramID))
		if err == nil {
			_ = h.store.SetTelegramID(ctx, user.ID, body.TelegramID)
		}
	}
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	if user == nil {
		username := body.Username
		if username == "" {
			username = fmt.Sprintf("tg_%d", body.TelegramID)
		}
		if body.FirstName != "" && username == fmt.Sprintf("tg_%d", body.TelegramID) {
			username = sanitizeUsername(body.FirstName, body.TelegramID)
		}

		panelUUID := newUUID()
		if err := h.x3dxui.CreateUser(ctx, username, 0, 0, panelUUID); err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": "failed to provision vpn user: " + err.Error()})
			return
		}

		_, pwdHash := auth.NewRefreshToken()
		user, err = h.store.CreateUser(ctx, username, "", pwdHash)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create user"})
			return
		}
		if err := h.store.SetTelegramID(ctx, user.ID, body.TelegramID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to link telegram account"})
			return
		}
		if err := h.store.SetPanelUsername(ctx, user.ID, username); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}
		if err := h.store.SetPanelUUID(ctx, user.ID, panelUUID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}
	}

	if !user.PanelUsername.Valid || !user.PanelUUID.Valid {
		c.JSON(http.StatusConflict, gin.H{"error": "vpn not provisioned"})
		return
	}

	resp := botUserResponse{
		TelegramID:  body.TelegramID,
		Username:    user.PanelUsername.String,
		Provisioned: true,
	}

	link, err := h.x3dxui.GetSubscriptionLink(ctx, user.PanelUsername.String)
	if err == nil {
		resp.SubscriptionURL = link
	}
	vless, err := h.x3dxui.BuildVLESSConfig(ctx, user.PanelUsername.String, user.PanelUUID.String)
	if err == nil {
		resp.VLESS = vless
	}
	singbox, err := h.x3dxui.BuildSingBoxConfig(ctx, user.PanelUsername.String, user.PanelUUID.String)
	if err == nil {
		resp.SingBox = singbox
	}

	c.JSON(http.StatusOK, resp)
}

// getBotUser returns the current status/config for a Telegram-backed user.
func (h *Handler) getBotUser(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	ctx := c.Request.Context()
	user, err := h.store.GetUserByTelegramID(ctx, id)
	if errors.Is(err, store.ErrNotFound) {
		user, err = h.store.GetUserByUsername(ctx, fmt.Sprintf("tg_%d", id))
	}
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	if !user.PanelUsername.Valid {
		c.JSON(http.StatusOK, botUserResponse{TelegramID: id, Username: user.Username, Provisioned: false})
		return
	}

	resp := botUserResponse{TelegramID: id, Username: user.PanelUsername.String, Provisioned: true}
	if link, err := h.x3dxui.GetSubscriptionLink(ctx, user.PanelUsername.String); err == nil {
		resp.SubscriptionURL = link
	}
	if vless, err := h.x3dxui.BuildVLESSConfig(ctx, user.PanelUsername.String, user.PanelUUID.String); err == nil {
		resp.VLESS = vless
	}
	if singbox, err := h.x3dxui.BuildSingBoxConfig(ctx, user.PanelUsername.String, user.PanelUUID.String); err == nil {
		resp.SingBox = singbox
	}

	c.JSON(http.StatusOK, resp)
}

// expiringBotUsers lists Telegram-backed clients whose subscription expires
// within the given window so the bot can send reminders.
func (h *Handler) expiringBotUsers(c *gin.Context) {
	hours := 72
	if v := c.Query("hours"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			hours = n
		}
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()

	clients, err := h.x3dxui.ListClients(ctx)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to list clients"})
		return
	}

	now := time.Now().UnixMilli()
	window := int64(hours) * 3600 * 1000
	out := make([]botExpiringResponse, 0, len(clients))

	for _, cl := range clients {
		if cl.ExpiryTime <= 0 {
			continue
		}
		if cl.ExpiryTime < now || cl.ExpiryTime > now+window {
			continue
		}
		user, err := h.store.GetUserByUsername(ctx, cl.Email)
		if err != nil || !user.TelegramID.Valid {
			continue
		}
		out = append(out, botExpiringResponse{
			TelegramID: user.TelegramID.Int64,
			Username:   cl.Email,
			ExpiresAt:  cl.ExpiryTime,
		})
	}

	c.JSON(http.StatusOK, gin.H{"users": out})
}

func sanitizeUsername(name string, tgID int64) string {
	out := make([]rune, 0, len(name))
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '.' || r == '-' {
			out = append(out, r)
		}
	}
	s := string(out)
	if len(s) < 3 {
		s = fmt.Sprintf("tg_%d", tgID)
	}
	if len(s) > 32 {
		s = s[:32]
	}
	return s
}
