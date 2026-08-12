package config

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	AppEnv      string
	Port        string
	DatabaseURL string
	RedisURL    string
	JWTSecret   string
	JWTPrivateKey  string
	CORSDomain     string
	X3dxuiURL         string
	X3dxuiUser        string
	X3dxuiPass        string
	X3dxuiInbound     string
	PublicOrigin   string
	BotToken      string
	AdminAPISecret string
}

func Load() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("/app/config")
	viper.AddConfigPath("./config")
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		// Config file is optional; we rely on environment variables.
		fmt.Printf("config file not loaded (using env): %v\n", err)
	}

	cfg := &Config{
		AppEnv:         viper.GetString("app_env"),
		Port:           viper.GetString("port"),
		DatabaseURL:    viper.GetString("database_url"),
		RedisURL:       viper.GetString("redis_url"),
		JWTSecret:      viper.GetString("jwt_secret"),
		CORSDomain:     viper.GetString("cors_domain"),
		X3dxuiURL:         viper.GetString("x3dxui_url"),
		X3dxuiUser:        viper.GetString("x3dxui_admin_username"),
		X3dxuiPass:        viper.GetString("x3dxui_admin_password"),
		X3dxuiInbound:     viper.GetString("x3dxui_inbound"),
		PublicOrigin:   viper.GetString("public_origin"),
		BotToken: viper.GetString("bot_token"),
		AdminAPISecret: viper.GetString("admin_api_secret"),
	}

	if cfg.Port == "" {
		cfg.Port = os.Getenv("PORT")
	}
	if cfg.Port == "" {
		cfg.Port = "8080"
	}
	if cfg.AppEnv == "" {
		cfg.AppEnv = os.Getenv("APP_ENV")
	}
	if cfg.AppEnv == "" {
		cfg.AppEnv = "development"
	}

	var missing []string
	if cfg.DatabaseURL == "" {
		missing = append(missing, "DATABASE_URL")
	}
	if cfg.RedisURL == "" {
		missing = append(missing, "REDIS_URL")
	}
	if cfg.JWTSecret == "" {
		missing = append(missing, "JWT_SECRET")
	}
	if cfg.X3dxuiURL == "" {
		missing = append(missing, "3DXUI_URL")
	}
	if cfg.X3dxuiUser == "" {
		missing = append(missing, "3DXUI_ADMIN_USERNAME")
	}
	if cfg.X3dxuiPass == "" {
		missing = append(missing, "X3DXUI_ADMIN_PASSWORD")
	}
	if cfg.BotToken == "" {
		missing = append(missing, "BOT_TOKEN")
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required config: %s", strings.Join(missing, ", "))
	}

	return cfg, nil
}

// IsProd reports whether the service runs in production mode.
func (c *Config) IsProd() bool { return c.AppEnv == "production" }

// CorsOrigins returns the list of allowed CORS origins.
func (c *Config) CorsOrigins() []string {
	if c.CORSDomain == "" {
		return nil
	}
	parts := strings.Split(c.CORSDomain, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

var ErrMissingConfig = errors.New("missing required configuration")
