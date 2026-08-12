package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type xhttpModeRequest struct {
	Mode string `json:"mode"`
}

func (h *Handler) updateXHTTPMode(c *gin.Context) {
	var body xhttpModeRequest
	if err := c.ShouldBindJSON(&body); err != nil || body.Mode == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 20*time.Second)
	defer cancel()

	if err := h.x3dxui.UpdateTransportMode(ctx, body.Mode); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to update mode: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"mode": body.Mode})
}
