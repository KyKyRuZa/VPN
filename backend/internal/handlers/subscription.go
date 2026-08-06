package handlers

import (
	"context"
	"net/http"
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
	if user.PanelUsername == "" {
		c.JSON(http.StatusConflict, gin.H{"error": "vpn not provisioned"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 12*time.Second)
	defer cancel()

	link, err := h.x3dxui.GetSubscriptionLink(ctx, user.PanelUsername)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to load subscription"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"subscription_url": link,
		"username":         user.PanelUsername,
	})
}
