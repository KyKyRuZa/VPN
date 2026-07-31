package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// MarzbanService talks to the Marzban API inside the docker network.
type MarzbanService struct {
	baseURL      string
	adminUser    string
	adminPass    string
	inboundTag   string
	publicOrigin string

	client *http.Client

	mu          sync.Mutex
	token       string
	tokenExpiry time.Time
}

// NewMarzbanService constructs the client. inboundTag is the Marzban inbound
// name used when provisioning users.
func NewMarzbanService(baseURL, adminUser, adminPass, inboundTag, publicOrigin string) *MarzbanService {
	if inboundTag == "" {
		inboundTag = "VLESS Reality"
	}
	return &MarzbanService{
		baseURL:      strings.TrimRight(baseURL, "/"),
		adminUser:    adminUser,
		adminPass:    adminPass,
		inboundTag:   inboundTag,
		publicOrigin: strings.TrimRight(publicOrigin, "/"),
		client:       &http.Client{Timeout: 15 * time.Second},
	}
}

type adminToken struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	TokenType   string `json:"token_type"`
}

// getToken returns a cached admin token, logging in when necessary.
func (s *MarzbanService) getToken(ctx context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.token != "" && time.Now().Before(s.tokenExpiry) {
		return s.token, nil
	}

	form := url.Values{}
	form.Set("username", s.adminUser)
	form.Set("password", s.adminPass)
	form.Set("scope", "")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+"/api/admin/token", strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("marzban login: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("marzban login failed: %d %s", resp.StatusCode, string(body))
	}

	var t adminToken
	if err := json.NewDecoder(resp.Body).Decode(&t); err != nil {
		return "", err
	}
	s.token = t.AccessToken
	ttl := time.Duration(t.ExpiresIn) * time.Second
	if ttl <= 0 {
		ttl = time.Hour
	}
	s.tokenExpiry = time.Now().Add(ttl - 30*time.Second)
	return s.token, nil
}

// CreateUser provisions a user in Marzban. expire is a unix timestamp (0 = never).
func (s *MarzbanService) CreateUser(ctx context.Context, username string, expire int64, dataLimitGB int) error {
	tok, err := s.getToken(ctx)
	if err != nil {
		return err
	}

	body := map[string]any{
		"username":   username,
		"expire":     expire,
		"data_limit": dataLimitGB * 1024 * 1024 * 1024,
		"proxies":    map[string]any{"vless": []map[string]any{{"id": "", "flow": "xtls-rprx-vision"}}},
		"inbounds":   map[string]any{s.inboundTag: []string{s.inboundTag}},
	}
	return s.do(ctx, http.MethodPost, "/api/user", tok, body, nil)
}

// GetSubscriptionLink returns the user's subscription URL (rewritten to the
// public origin when configured).
func (s *MarzbanService) GetSubscriptionLink(ctx context.Context, username string) (string, error) {
	tok, err := s.getToken(ctx)
	if err != nil {
		return "", err
	}
	var out struct {
		SubscriptionURL string `json:"subscription_url"`
	}
	if err := s.do(ctx, http.MethodGet, "/api/user/"+url.PathEscape(username)+"/subscription/link", tok, nil, &out); err != nil {
		return "", err
	}
	if s.publicOrigin == "" || out.SubscriptionURL == "" {
		return out.SubscriptionURL, nil
	}
	u, err := url.Parse(out.SubscriptionURL)
	if err != nil {
		return out.SubscriptionURL, nil
	}
	return s.publicOrigin + u.Path, nil
}

func (s *MarzbanService) do(ctx context.Context, method, path, token string, reqBody any, out any) error {
	var rdr io.Reader
	if reqBody != nil {
		b, err := json.Marshal(reqBody)
		if err != nil {
			return err
		}
		rdr = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, s.baseURL+path, rdr)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if reqBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("marzban request: %w", err)
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("marzban %s %s: %d %s", method, path, resp.StatusCode, string(data))
	}
	if out != nil && len(data) > 0 {
		return json.Unmarshal(data, out)
	}
	return nil
}
