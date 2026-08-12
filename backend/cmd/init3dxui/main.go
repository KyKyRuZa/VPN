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

	"golang.org/x/crypto/curve25519"
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
		inboundTag = "vless-reality-tcp"
	}
	publicHost := os.Getenv("X3DXUI_PUBLIC_HOST")

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

	var existingIB map[string]any
	for _, ib := range inbounds {
		if strings.EqualFold(ib["tag"].(string), inboundTag) {
			existingIB = ib
			break
		}
	}

	if existingIB != nil {
		fmt.Printf("updating inbound %d to ensure correct settings\n", int(existingIB["id"].(float64)))
		if err := updateInboundSettings(ctx, client, cookies, csrf, existingIB, publicHost); err != nil {
			fmt.Println("failed to update inbound:", err)
			os.Exit(1)
		}
		fmt.Println("ok: inbound updated")
	} else {
		if err := createInbound(ctx, client, cookies, csrf, inboundTag, publicHost); err != nil {
			fmt.Println("failed to create inbound:", err)
			os.Exit(1)
		}
		fmt.Println("ok: inbound created")
	}

	if err := patchExistingClients(ctx, client, cookies, csrf); err != nil {
		fmt.Println("failed to patch clients:", err)
		os.Exit(1)
	}
	fmt.Println("ok: clients patched")
}

func buildStreamSettings(publicHost, privateKey, publicKey, shortID string) map[string]any {
	rs := map[string]any{
		"dest":                   "www.microsoft.com:443",
		"serverNames":            []string{"www.microsoft.com"},
		"fingerprint":            "chrome",
		"spx":                    "%2F",
		"show":                   false,
		"settings":               map[string]any{},
		"minimal_client_version": "",
	}
	if privateKey != "" && publicKey != "" {
		rs["privateKey"] = privateKey
		rs["publicKey"] = publicKey
	}
	if shortID != "" {
		rs["shortIds"] = []string{shortID}
	}

	return map[string]any{
		"network":         "tcp",
		"security":        "reality",
		"realitySettings": rs,
		"sockopt": map[string]any{
			"tcpFastOpen":   true,
			"tcpcongestion": "bbr",
		},
	}
}

func updateInboundSettings(ctx context.Context, client *http.Client, cookies map[string]string, csrf string, existingIB map[string]any, publicHost string) error {
	id := int(existingIB["id"].(float64))
	existingIB["port"] = 8433

	streamSettings := existingIB["streamSettings"]
	if ss, ok := streamSettings.(map[string]any); ok {
		ss["network"] = "tcp"
		ss["security"] = "reality"

		realitySettings := ss["realitySettings"]
		if rs, ok := realitySettings.(map[string]any); ok {
			if _, hasPrivate := rs["privateKey"].(string); !hasPrivate {
				privateKey, publicKey, err := generateRealityKeys(ctx, client, cookies, csrf)
				if err != nil {
					return err
				}
				rs["privateKey"] = privateKey
				rs["publicKey"] = publicKey
				if _, hasShort := rs["shortIds"].([]any); !hasShort {
					rs["shortIds"] = []string{randomHex(8)}
				}
			} else if _, hasPublic := rs["publicKey"].(string); !hasPublic {
				rs["publicKey"] = derivePublicKey(rs["privateKey"].(string))
			}
			rs["dest"] = "www.microsoft.com:443"
			rs["serverNames"] = []string{"www.microsoft.com"}
			rs["fingerprint"] = "chrome"
			rs["spx"] = "%2F"
			rs["show"] = false
			rs["minimal_client_version"] = ""
		} else {
			privateKey, publicKey, err := generateRealityKeys(ctx, client, cookies, csrf)
			if err != nil {
				return err
			}
			shortID := randomHex(8)
			ss["realitySettings"] = map[string]any{
				"dest":                   "www.microsoft.com:443",
				"serverNames":            []string{"www.microsoft.com"},
				"fingerprint":            "chrome",
				"spx":                    "%2F",
				"show":                   false,
				"settings":               map[string]any{},
				"privateKey":             privateKey,
				"publicKey":              publicKey,
				"shortIds":               []string{shortID},
				"minimal_client_version": "",
			}
		}

		sockopt := ss["sockopt"]
		if so, ok := sockopt.(map[string]any); ok {
			so["tcpFastOpen"] = true
			so["tcpcongestion"] = "bbr"
		} else {
			ss["sockopt"] = map[string]any{
				"tcpFastOpen":   true,
				"tcpcongestion": "bbr",
			}
		}
	} else {
		privateKey, publicKey, err := generateRealityKeys(ctx, client, cookies, csrf)
		if err != nil {
			return err
		}
		shortID := randomHex(8)
		existingIB["streamSettings"] = buildStreamSettings(publicHost, privateKey, publicKey, shortID)
	}

	existingIB["sniffing"] = map[string]any{
		"enabled":      true,
		"destOverride": []string{"http", "tls"},
	}

	if settings, ok := existingIB["settings"].(map[string]any); ok {
		settings["flow"] = "xtls-rprx-vision"
	}

	b, _ := json.Marshal(existingIB)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/panel/api/inbounds/update/%d", baseURL, id), bytes.NewReader(b))
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
		return fmt.Errorf("update inbound status %d: %s", resp.StatusCode, string(data))
	}

	// 3x-ui в списке инбаундов не отдаёт publicKey; убедимся, что он записан.
	full, err := getInboundByID(ctx, client, cookies, id)
	if err == nil {
		if ss, ok := full["streamSettings"].(map[string]any); ok {
			if rs, ok := ss["realitySettings"].(map[string]any); ok {
				priv, _ := rs["privateKey"].(string)
				pub, _ := rs["publicKey"].(string)
				if priv != "" && pub == "" {
					rs["publicKey"] = derivePublicKey(priv)
					full["id"] = id
					if b, err := json.Marshal(full); err == nil {
						req2, _ := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/panel/api/inbounds/update/%d", baseURL, id), bytes.NewReader(b))
						req2.Header.Set("Content-Type", "application/json")
						req2.Header.Set("X-CSRF-Token", csrf)
						for k, v := range cookies {
							req2.Header.Set(k, v)
						}
						if resp2, err2 := client.Do(req2); err2 == nil {
							resp2.Body.Close()
						}
					}
				}
			}
		}
	}

	return nil
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

func getInboundByID(ctx context.Context, client *http.Client, cookies map[string]string, id int) (map[string]any, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/panel/api/inbounds/get/%d", baseURL, id), nil)
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
		return nil, fmt.Errorf("get inbound %d status %d: %s", id, resp.StatusCode, string(data))
	}
	var out struct {
		Obj map[string]any `json:"obj"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	if out.Obj == nil {
		return nil, fmt.Errorf("inbound %d not found in response", id)
	}
	return out.Obj, nil
}

func createInbound(ctx context.Context, client *http.Client, cookies map[string]string, csrf, inboundTag, publicHost string) error {
	privateKey, publicKey, err := generateRealityKeys(ctx, client, cookies, csrf)
	if err != nil {
		return err
	}
	shortID := randomHex(8)

	payload := map[string]any{
		"enable":     true,
		"remark":     "VLESS Reality TCP",
		"listen":     "",
		"port":       8433,
		"protocol":   "vless",
		"expiryTime": 0,
		"total":      0,
		"settings": map[string]any{
			"clients":    []map[string]any{},
			"decryption": "none",
			"fallbacks":  []map[string]any{},
			"flow":       "xtls-rprx-vision",
		},
		"streamSettings": buildStreamSettings(publicHost, privateKey, publicKey, shortID),
		"sniffing": map[string]any{
			"enabled":      true,
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

func derivePublicKey(privateKey string) string {
	sk := make([]byte, curve25519.ScalarSize)
	if _, err := hex.Decode(sk, []byte(privateKey)); err != nil {
		return ""
	}
	var pk [curve25519.PointSize]byte
	curve25519.ScalarBaseMult(&pk, (*[curve25519.ScalarSize]byte)(sk))
	return hex.EncodeToString(pk[:])
}

func patchExistingClients(ctx context.Context, client *http.Client, cookies map[string]string, csrf string) error {
	clients, err := listClients(ctx, client, cookies)
	if err != nil {
		return fmt.Errorf("list clients: %w", err)
	}

	for _, c := range clients {
		email, _ := c["email"].(string)
		id, _ := c["id"].(string)
		if email == "" || id == "" {
			continue
		}

		password, _ := c["password"].(string)
		flow, _ := c["flow"].(string)
		if password != "" && flow == "xtls-rprx-vision" {
			continue
		}

		updated := map[string]any{
			"id":       id,
			"email":    email,
			"password": id,
			"flow":     "xtls-rprx-vision",
		}
		if v, ok := c["totalGB"].(float64); ok {
			updated["totalGB"] = v
		}
		if v, ok := c["expiryTime"].(float64); ok {
			updated["expiryTime"] = v
		}
		if v, ok := c["limitIp"].(float64); ok {
			updated["limitIp"] = v
		}
		if v, ok := c["enable"].(bool); ok {
			updated["enable"] = v
		}

		if err := updateClient(ctx, client, cookies, csrf, email, updated); err != nil {
			fmt.Printf("failed to update client %s: %v\n", email, err)
			continue
		}
		fmt.Printf("ok: patched client %s\n", email)
	}
	return nil
}

func listClients(ctx context.Context, client *http.Client, cookies map[string]string) ([]map[string]any, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/panel/api/clients/list", nil)
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
		return nil, fmt.Errorf("list clients status %d: %s", resp.StatusCode, string(data))
	}
	var out struct {
		Obj []map[string]any `json:"obj"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return out.Obj, nil
}

func updateClient(ctx context.Context, client *http.Client, cookies map[string]string, csrf, email string, body map[string]any) error {
	b, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/panel/api/clients/update/%s", baseURL, email), bytes.NewReader(b))
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
		return fmt.Errorf("update client status %d: %s", resp.StatusCode, string(data))
	}
	return nil
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
