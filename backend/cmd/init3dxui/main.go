package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

var baseURL = "http://localhost:80/panel"

type loginResponse struct {
	Success bool   `json:"success"`
	Msg     string `json:"msg"`
}

func main() {
	if envURL := os.Getenv("X3DXUI_URL"); envURL != "" {
		baseURL = envURL
	}
	adminUser := os.Getenv("X3DXUI_ADMIN_USERNAME")
	adminPass := os.Getenv("X3DXUI_ADMIN_PASSWORD")
	inboundTag := os.Getenv("X3DXUI_INBOUND")
	if inboundTag == "" {
		inboundTag = "vless-reality-xhttp"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client := &http.Client{Timeout: 15 * time.Second}

	if adminUser == "" || adminPass == "" {
		fmt.Println("skip: X3DXUI_ADMIN_USERNAME or X3DXUI_ADMIN_PASSWORD is not set")
		return
	}

	cookies, csrf, err := login(ctx, client, adminUser, adminPass)
	if err != nil {
		fmt.Println("failed to login with configured credentials:", err)
		if adminUser != "admin" || adminPass != "admin" {
			fmt.Println("retrying with fallback admin/admin ...")
			cookies, csrf, err = login(ctx, client, "admin", "admin")
			if err != nil {
				fmt.Println("failed to login with fallback credentials:", err)
				os.Exit(1)
			}
		} else {
			os.Exit(1)
		}
	}

	inbounds, err := listInbounds(ctx, client, cookies)
	if err != nil {
		fmt.Println("skip: cannot list inbounds:", err)
		return
	}
	for _, ib := range inbounds {
		if strings.EqualFold(ib["tag"].(string), inboundTag) {
			fmt.Println("skip: inbound already exists")
			return
		}
	}

	if err := createInbound(ctx, client, cookies, csrf, inboundTag); err != nil {
		fmt.Println("failed to create inbound:", err)
		os.Exit(1)
	}
	fmt.Println("ok: inbound created")
}

func login(ctx context.Context, client *http.Client, username, password string) (map[string]string, string, error) {
	jar := &cookieJar{}

	csrfReq, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/csrf-token", nil)
	if err != nil {
		return nil, "", err
	}
	csrfResp, err := client.Do(csrfReq)
	if err != nil {
		return nil, "", fmt.Errorf("3dxui csrf request: %w", err)
	}
	defer csrfResp.Body.Close()

	var csrfData struct {
		Obj     string `json:"obj"`
		Success bool   `json:"success"`
	}
	if err := json.NewDecoder(csrfResp.Body).Decode(&csrfData); err != nil {
		return nil, "", fmt.Errorf("3dxui csrf parse: %w", err)
	}
	if !csrfData.Success {
		return nil, "", fmt.Errorf("3dxui csrf: unsuccessful response")
	}

	for _, c := range csrfResp.Cookies() {
		jar.set(c.Name, c.Value)
	}

	loginBody, err := json.Marshal(map[string]string{
		"username": username,
		"password": password,
	})
	if err != nil {
		return nil, "", err
	}

	loginReq, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/login", bytes.NewReader(loginBody))
	if err != nil {
		return nil, "", err
	}
	loginReq.Header.Set("Content-Type", "application/json")
	loginReq.Header.Set("X-CSRF-Token", csrfData.Obj)
	loginReq.Header.Set("X-Requested-With", "XMLHttpRequest")
	for k, v := range jar.headers() {
		loginReq.Header.Set(k, v)
	}

	loginResp, err := client.Do(loginReq)
	if err != nil {
		return nil, "", fmt.Errorf("3dxui login request: %w", err)
	}
	defer loginResp.Body.Close()

	body, _ := io.ReadAll(loginResp.Body)
	if loginResp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("3dxui login failed: %d %s", loginResp.StatusCode, string(body))
	}

	var lr loginResponse
	if err := json.Unmarshal(body, &lr); err != nil {
		return nil, "", fmt.Errorf("3dxui login parse: %w", err)
	}
	if !lr.Success {
		return nil, "", fmt.Errorf("3dxui login failed: %s", lr.Msg)
	}

	for _, c := range loginResp.Cookies() {
		jar.set(c.Name, c.Value)
	}
	return jar.headers(), csrfData.Obj, nil
}

func listInbounds(ctx context.Context, client *http.Client, cookies map[string]string) ([]map[string]any, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/panel/api/inbounds/list", nil)
	if err != nil {
		return nil, err
	}
	for k, v := range cookies {
		req.Header.Set(k, v)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list inbounds status %d: %s", resp.StatusCode, string(data))
	}
	var out struct {
		Obj []map[string]any `json:"obj"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return out.Obj, nil
}

func createInbound(ctx context.Context, client *http.Client, cookies map[string]string, csrf, inboundTag string) error {
	privateKey, publicKey, err := generateRealityKeys(ctx, client, cookies, csrf)
	if err != nil {
		return err
	}
	shortID := randomHex(8)

	payload := map[string]any{
		"enable": true,
		"remark": "VLESS Reality XHTTP",
		"listen": "",
		"port":   8443,
		"protocol": "vless",
		"expiryTime": 0,
		"total": 0,
		"settings": map[string]any{
			"clients":    []map[string]any{},
			"decryption": "none",
			"fallbacks":  []map[string]any{},
			"flow":       "xtls-rprx-vision",
		},
		"streamSettings": map[string]any{
			"network": "xhttp",
			"security": "reality",
			"realitySettings": map[string]any{
				"show": false,
				"dest": "www.microsoft.com:443",
				"serverNames": []string{"www.microsoft.com"},
				"privateKey": privateKey,
				"publicKey":  publicKey,
				"shortIds":   []string{shortID},
			},
			"xhttpSettings": map[string]any{
				"mode": "packet-up",
				"xPaddingBytes": "100-1000",
				"xPaddingObfsMode": true,
				"scMaxEachPostBytes": 1000000,
				"scMaxBufferedPosts": 30,
			},
			"sockopt": map[string]any{
				"tcpFastOpen": true,
				"tcpcongestion": "bbr",
			},
		},
		"sniffing": map[string]any{
			"enabled": true,
			"destOverride": []string{"http", "tls"},
		},
		"tag": inboundTag,
	}

	b, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/panel/api/inbounds/add", bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", csrf)
	for k, v := range cookies {
		req.Header.Set(k, v)
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("create inbound status %d: %s", resp.StatusCode, string(data))
	}
	return nil
}

func generateRealityKeys(ctx context.Context, client *http.Client, cookies map[string]string, csrf string) (privateKey, publicKey string, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/panel/api/server/getNewX25519Cert", nil)
	if err != nil {
		return "", "", err
	}
	for k, v := range cookies {
		req.Header.Set(k, v)
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("reality keys status %d: %s", resp.StatusCode, string(data))
	}
	var out struct {
		Obj struct {
			PrivateKey string `json:"privateKey"`
			PublicKey  string `json:"publicKey"`
		} `json:"obj"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return "", "", err
	}
	return out.Obj.PrivateKey, out.Obj.PublicKey, nil
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)[:n]
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
