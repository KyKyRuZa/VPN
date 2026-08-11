package services

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type x3dxuiSession struct {
	cookies map[string]string
	csrf    string
}

type cookieJar map[string]string

func (j *cookieJar) set(name, value string) {
	if j == nil {
		*j = make(cookieJar)
	}
	(*j)[name] = value
}

func (j *cookieJar) headers() map[string]string {
	if j == nil || len(*j) == 0 {
		return nil
	}
	h := make(map[string]string, len(*j))
	var parts []string
	for k, v := range *j {
		parts = append(parts, fmt.Sprintf("%s=%s", k, v))
	}
	h["Cookie"] = strings.Join(parts, "; ")
	return h
}

// X3dxuiService talks to the 3x-ui API inside the docker network.
type X3dxuiService struct {
	baseURL      string
	adminUser    string
	adminPass    string
	inboundTag   string
	publicOrigin string

	client *http.Client

	mu             sync.Mutex
	session        *x3dxuiSession
	lastAuth       time.Time
	inboundID      int
	inboundIDFound bool
}

func newUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func normalizeBase64(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}

	if decoded, err := base64.StdEncoding.DecodeString(s); err == nil && len(decoded) == 32 {
		return base64.RawURLEncoding.EncodeToString(decoded)
	}
	if decoded, err := base64.RawURLEncoding.DecodeString(s); err == nil && len(decoded) == 32 {
		return s
	}
	if decoded, err := hex.DecodeString(s); err == nil && len(decoded) == 32 {
		return base64.RawURLEncoding.EncodeToString(decoded)
	}

	s = strings.ReplaceAll(s, "+", "-")
	s = strings.ReplaceAll(s, "/", "_")
	s = strings.TrimRight(s, "=")
	return s
}

func NewX3dxuiService(baseURL, adminUser, adminPass, inboundTag, publicOrigin string) *X3dxuiService {
	if inboundTag == "" {
		inboundTag = "vless-reality-xhttp"
	}
	return &X3dxuiService{
		baseURL:      strings.TrimRight(baseURL, "/"),
		adminUser:    adminUser,
		adminPass:    adminPass,
		inboundTag:   inboundTag,
		publicOrigin: strings.TrimRight(publicOrigin, "/"),
		client:       &http.Client{Timeout: 15 * time.Second},
	}
}

type loginResponse struct {
	Success bool   `json:"success"`
	Msg     string `json:"msg"`
}

func (s *X3dxuiService) ensureSession(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.session != nil && time.Since(s.lastAuth) < time.Hour {
		return nil
	}

	users := []struct {
		user string
		pass string
	}{{s.adminUser, s.adminPass}, {"admin", "admin"}}

	var lastErr error
	for _, u := range users {
		if u.user == "" || u.pass == "" {
			continue
		}
		session, err := s.login(ctx, u.user, u.pass)
		if err == nil {
			s.session = session
			s.lastAuth = time.Now()
			return nil
		}
		lastErr = err
	}
	return fmt.Errorf("3dxui login: %w", lastErr)
}

func (s *X3dxuiService) login(ctx context.Context, username, password string) (*x3dxuiSession, error) {
	jar := &cookieJar{}
	csrfReq, err := http.NewRequestWithContext(ctx, http.MethodGet, s.baseURL+"/csrf-token", nil)
	if err != nil {
		return nil, err
	}
	csrfResp, err := s.client.Do(csrfReq)
	if err != nil {
		return nil, fmt.Errorf("3dxui csrf request: %w", err)
	}
	defer csrfResp.Body.Close()

	var csrfData struct {
		Obj     string `json:"obj"`
		Success bool   `json:"success"`
	}
	if err := json.NewDecoder(csrfResp.Body).Decode(&csrfData); err != nil {
		return nil, fmt.Errorf("3dxui csrf parse: %w", err)
	}
	if !csrfData.Success {
		return nil, fmt.Errorf("3dxui csrf: unsuccessful response")
	}

	for _, c := range csrfResp.Cookies() {
		jar.set(c.Name, c.Value)
	}

	loginBody, err := json.Marshal(map[string]string{
		"username": username,
		"password": password,
	})
	if err != nil {
		return nil, err
	}

	loginReq, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+"/login", bytes.NewReader(loginBody))
	if err != nil {
		return nil, err
	}
	loginReq.Header.Set("Content-Type", "application/json")
	loginReq.Header.Set("X-CSRF-Token", csrfData.Obj)
	loginReq.Header.Set("X-Requested-With", "XMLHttpRequest")
	for k, v := range jar.headers() {
		loginReq.Header.Set(k, v)
	}

	loginResp, err := s.client.Do(loginReq)
	if err != nil {
		return nil, fmt.Errorf("3dxui login request: %w", err)
	}
	defer loginResp.Body.Close()

	body, _ := io.ReadAll(loginResp.Body)
	if loginResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("3dxui login failed: %d %s", loginResp.StatusCode, string(body))
	}

	var lr loginResponse
	if err := json.Unmarshal(body, &lr); err != nil {
		return nil, fmt.Errorf("3dxui login parse: %w", err)
	}
	if !lr.Success {
		return nil, fmt.Errorf("3dxui login failed: %s", lr.Msg)
	}

	for _, c := range loginResp.Cookies() {
		jar.set(c.Name, c.Value)
	}
	return &x3dxuiSession{cookies: jar.headers(), csrf: csrfData.Obj}, nil
}

func (s *X3dxuiService) do(ctx context.Context, method, path string, reqBody any, out any) error {
	if err := s.ensureSession(ctx); err != nil {
		return fmt.Errorf("ensureSession: %w", err)
	}

	var rdr io.Reader
	if reqBody != nil {
		b, err := json.Marshal(reqBody)
		if err != nil {
			return err
		}
		rdr = bytes.NewReader(b)
	}

	fullURL := s.baseURL + path
	req, err := http.NewRequestWithContext(ctx, method, fullURL, rdr)
	if err != nil {
		return err
	}
	for k, v := range s.session.cookies {
		req.Header.Set(k, v)
	}
	if s.session.csrf != "" && (method == http.MethodPost || method == http.MethodPut || method == http.MethodPatch || method == http.MethodDelete) {
		req.Header.Set("X-CSRF-Token", s.session.csrf)
	}
	if reqBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Duration(attempt) * 200 * time.Millisecond):
			}
		}

		resp, err := s.client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("3dxui request: %w", err)
			continue
		}
		defer resp.Body.Close()

		data, _ := io.ReadAll(resp.Body)
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			lastErr = fmt.Errorf("3dxui %s %s: %d %s", method, path, resp.StatusCode, string(data))
			if resp.StatusCode == http.StatusUnauthorized {
				s.mu.Lock()
				s.session = nil
				s.mu.Unlock()
			}
			continue
		}
		if out != nil && len(data) > 0 {
			if err := json.Unmarshal(data, out); err != nil {
				return fmt.Errorf("3dxui %s %s: json unmarshal: %w, body: %s", method, path, err, string(data))
			}
			return nil
		}
		return nil
	}
	return lastErr
}

func (s *X3dxuiService) getInboundID(ctx context.Context) (int, error) {
	s.mu.Lock()
	if s.inboundIDFound {
		id := s.inboundID
		s.mu.Unlock()
		return id, nil
	}
	s.mu.Unlock()

	var out struct {
		Obj []struct {
			ID  int    `json:"id"`
			Tag string `json:"tag"`
		} `json:"obj"`
	}
	if err := s.do(ctx, http.MethodGet, "/panel/api/inbounds/list", nil, &out); err != nil {
		return 0, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	for _, ib := range out.Obj {
		if strings.EqualFold(ib.Tag, s.inboundTag) {
			s.inboundID = ib.ID
			s.inboundIDFound = true
			return ib.ID, nil
		}
	}
	return 0, fmt.Errorf("inbound %q not found", s.inboundTag)
}

// CreateUser provisions a user in 3x-ui.
func (s *X3dxuiService) CreateUser(ctx context.Context, username string, expire int64, dataLimitGB int, uuid string) error {
	inboundID, err := s.getInboundID(ctx)
	if err != nil {
		return fmt.Errorf("getInboundID: %w", err)
	}

	body := map[string]any{
		"client": map[string]any{
			"email":      username,
			"id":         uuid,
			"totalGB":    dataLimitGB * 1024 * 1024 * 1024,
			"expiryTime": expire * 1000,
			"limitIp":    0,
			"enable":     true,
		},
		"inboundIds": []int{inboundID},
	}
	if err := s.do(ctx, http.MethodPost, "/panel/api/clients/add", body, nil); err != nil {
		return fmt.Errorf("clients/add: %w", err)
	}
	return nil
}

// GetSubscriptionLink returns the user's subscription URL.
func (s *X3dxuiService) GetSubscriptionLink(ctx context.Context, username string) (string, error) {
	var clientOut struct {
		Obj struct {
			Client struct {
				SubId string `json:"subId"`
			} `json:"client"`
		} `json:"obj"`
	}
	if err := s.do(ctx, http.MethodGet, "/panel/api/clients/get/"+url.PathEscape(username), nil, &clientOut); err != nil {
		return "", err
	}

	subId := clientOut.Obj.Client.SubId
	if subId == "" {
		return "", fmt.Errorf("subscription id not found for %s", username)
	}

	if s.publicOrigin != "" {
		return fmt.Sprintf("%s/sub/%s", s.publicOrigin, subId), nil
	}
	return fmt.Sprintf("/sub/%s", subId), nil
}

// GetInboundConfig returns the first matching inbound config.
func (s *X3dxuiService) GetInboundConfig(ctx context.Context) (map[string]any, error) {
	var out struct {
		Obj []map[string]any `json:"obj"`
	}
	if err := s.do(ctx, http.MethodGet, "/panel/api/inbounds/list", nil, &out); err != nil {
		return nil, err
	}
	for _, ib := range out.Obj {
		if strings.EqualFold(ib["tag"].(string), s.inboundTag) {
			return ib, nil
		}
	}
	return nil, fmt.Errorf("inbound %q not found", s.inboundTag)
}

// BuildVLESSConfig builds a base64 VLESS config for the given client.
func (s *X3dxuiService) BuildVLESSConfig(ctx context.Context, username, uuid string) (string, error) {
	ib, err := s.GetInboundConfig(ctx)
	if err != nil {
		return "", err
	}

	port := int(ib["port"].(float64))
	stream := ib["streamSettings"].(map[string]any)
	reality := stream["realitySettings"].(map[string]any)
	publicKey := normalizeBase64(reality["publicKey"].(string))
	serverNames := reality["serverNames"].([]any)
	sni := serverNames[0].(string)
	shortIds := reality["shortIds"].([]any)
	shortID := ""
	if len(shortIds) > 0 {
		shortID = shortIds[0].(string)
	}

	host := s.publicOrigin
	if host == "" {
		host = sni
	}
	if strings.HasPrefix(host, "https://") {
		host = strings.TrimPrefix(host, "https://")
	} else if strings.HasPrefix(host, "http://") {
		host = strings.TrimPrefix(host, "http://")
	}

	link := fmt.Sprintf(
		"vless://%s@%s:%d?encryption=none&flow=xtls-rprx-vision&host=%s&mode=packet-up&path=&pbk=%s&security=reality&sid=%s&sni=%s&spx=%s&type=xhttp&fp=chrome&x_padding_bytes=100-1000#%s",
		uuid,
		host,
		port,
		host,
		publicKey,
		shortID,
		sni,
		"",
		url.QueryEscape("VLESS Reality XHTTP-"+username),
	)

	return link, nil
}

// BuildSingBoxConfig builds a sing-box JSON config for the given client.
func (s *X3dxuiService) BuildSingBoxConfig(ctx context.Context, username, uuid string) (string, error) {
	ib, err := s.GetInboundConfig(ctx)
	if err != nil {
		return "", err
	}

	port := int(ib["port"].(float64))
	stream := ib["streamSettings"].(map[string]any)
	reality := stream["realitySettings"].(map[string]any)
	publicKey := normalizeBase64(reality["publicKey"].(string))
	serverNames := reality["serverNames"].([]any)
	sni := serverNames[0].(string)
	shortIds := reality["shortIds"].([]any)
	shortID := ""
	if len(shortIds) > 0 {
		shortID = shortIds[0].(string)
	}

	host := s.publicOrigin
	if host == "" {
		host = sni
	}
	if strings.HasPrefix(host, "https://") {
		host = strings.TrimPrefix(host, "https://")
	} else if strings.HasPrefix(host, "http://") {
		host = strings.TrimPrefix(host, "http://")
	}

	config := map[string]any{
		"version": 1,
		"outbounds": []map[string]any{
			{
				"type":        "vless",
				"server":      host,
				"server_port": port,
				"uuid":        uuid,
				"password":    uuid,
				"flow":        "xtls-rprx-vision",
				"transport": map[string]any{
					"type": "xhttp",
					"host": host,
					"path": "/",
					"mode": "packet-up",
				},
				"tls": map[string]any{
					"enabled":     true,
					"server_name": sni,
					"fingerprint": "chrome",
				"reality": map[string]any{
					"pbk":        publicKey,
					"short_id":   shortID,
				},
				},
			},
		},
	}

	configJSON, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return "", err
	}

	return string(configJSON), nil
}
