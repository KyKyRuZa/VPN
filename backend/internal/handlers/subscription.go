package handlers

import (
	"net/http"

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
	if user.MarzbanUsername == "" {
		c.JSON(http.StatusConflict, gin.H{"error": "vpn not provisioned"})
		return
	}

	link, err := h.marzban.GetSubscriptionLink(c.Request.Context(), user.MarzbanUsername)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to load subscription"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"subscription_url": link,
		"username":         user.MarzbanUsername,
	})
}
