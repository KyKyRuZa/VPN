package handlers

import (
	"context"
	"net/http"
	"net/url"
	"time"

	"github.com/gin-gonic/gin"
)

func (h *Handler) subscription(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	user, err := h.store.GetUserByID(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	if !user.PanelUsername.Valid {
		c.JSON(http.StatusConflict, gin.H{"error": "vpn not provisioned"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 12*time.Second)
	defer cancel()

	link, err := h.x3dxui.GetSubscriptionLink(ctx, user.PanelUsername.String)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to load subscription"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"subscription_url": link,
		"username":         user.PanelUsername.String,
	})
}

func (h *Handler) subscriptionConfig(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	user, err := h.store.GetUserByID(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	if !user.PanelUUID.Valid {
		c.JSON(http.StatusConflict, gin.H{"error": "vpn not provisioned"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 12*time.Second)
	defer cancel()

	link, err := h.x3dxui.BuildVLESSConfig(ctx, user.PanelUsername.String, user.PanelUUID.String)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to build config"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"config_url": link,
		"username":   user.PanelUsername.String,
	})
}

func (h *Handler) subscriptionConfigQR(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	user, err := h.store.GetUserByID(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	if !user.PanelUUID.Valid {
		c.JSON(http.StatusConflict, gin.H{"error": "vpn not provisioned"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 12*time.Second)
	defer cancel()

	link, err := h.x3dxui.BuildVLESSConfig(ctx, user.PanelUsername.String, user.PanelUUID.String)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to build config"})
		return
	}

	c.Redirect(http.StatusFound, "https://api.qrserver.com/v1/create-qr-code/?size=400x400&data="+url.QueryEscape(link))
}
