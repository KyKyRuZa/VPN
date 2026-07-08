package handlers

import (
	"github.com/gin-gonic/gin"

	"github.com/ilyas/vpn-service/backend/internal/auth"
	"github.com/ilyas/vpn-service/backend/internal/config"
	"github.com/ilyas/vpn-service/backend/internal/middleware"
	"github.com/ilyas/vpn-service/backend/internal/services"
	"github.com/ilyas/vpn-service/backend/internal/store"
)

// Handler holds dependencies shared by all HTTP handlers.
type Handler struct {
	store   *store.Store
	jwt     *auth.TokenService
	marzban *services.MarzbanService
	cfg     *config.Config
}

func NewHandler(s *store.Store, j *auth.TokenService, m *services.MarzbanService, cfg *config.Config) *Handler {
	return &Handler{store: s, jwt: j, marzban: m, cfg: cfg}
}

// RegisterRoutes wires all application routes onto the engine.
func (h *Handler) RegisterRoutes(r *gin.Engine) {
	r.GET("/health", h.health)
	api := r.Group("/api")
	{
		api.GET("/health", h.health)

		auth := api.Group("/auth")
		{
			auth.POST("/register", h.register)
			auth.POST("/login", h.login)
			auth.POST("/refresh", h.refresh)
			auth.POST("/logout", h.logout)
			auth.GET("/profile", middleware.AuthRequired(h.jwt), h.profile)
			auth.PATCH("/profile", middleware.AuthRequired(h.jwt), h.updateProfile)
			auth.POST("/password", middleware.AuthRequired(h.jwt), h.changePassword)
		}

		api.GET("/subscription", middleware.AuthRequired(h.jwt), h.subscription)
	}
}

func (h *Handler) health(c *gin.Context) {
	c.JSON(200, gin.H{"status": "ok"})
}
