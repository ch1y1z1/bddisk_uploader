package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"bddisk_uploader/logger"
)

// OAuth相关常量
const (
	AuthURL            = "https://openapi.baidu.com/oauth/2.0/authorize"
	TokenURL           = "https://openapi.baidu.com/oauth/2.0/token"
	DefaultRedirectURI = "http://localhost:8080/callback"
	DefaultScope       = "basic,netdisk"
	CallbackPath       = "/callback"
)

// 授权响应结构体
type TokenResponse struct {
	AccessToken      string `json:"access_token"`
	RefreshToken     string `json:"refresh_token"`
	ExpiresIn        int    `json:"expires_in"`
	Scope            string `json:"scope"`
	SessionKey       string `json:"session_key"`
	SessionSecret    string `json:"session_secret"`
	Error            string `json:"error,omitempty"`
	ErrorDescription string `json:"error_description,omitempty"`
}

// OAuth配置
type OAuthConfig struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	RedirectURI  string `json:"redirect_uri"`
	Scope        string `json:"scope"`
}

// 生成授权URL
func generateAuthURL(config *OAuthConfig) string {
	params := url.Values{}
	params.Set("response_type", "code")
	params.Set("client_id", config.ClientID)
	params.Set("redirect_uri", config.RedirectURI)
	params.Set("scope", config.Scope)

	return AuthURL + "?" + params.Encode()
}

// 用授权码获取access_token
func getAccessToken(config *OAuthConfig, authCode string) (*TokenResponse, error) {
	params := url.Values{}
	params.Set("grant_type", "authorization_code")
	params.Set("code", authCode)
	params.Set("client_id", config.ClientID)
	params.Set("client_secret", config.ClientSecret)
	params.Set("redirect_uri", config.RedirectURI)

	resp, err := http.PostForm(TokenURL, params)
	if err != nil {
		return nil, fmt.Errorf("请求token失败: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %v", err)
	}

	var tokenResp TokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("解析响应失败: %v", err)
	}

	if tokenResp.Error != "" {
		return nil, fmt.Errorf("获取token失败: %s - %s", tokenResp.Error, tokenResp.ErrorDescription)
	}

	return &tokenResp, nil
}

// 刷新access_token
func refreshAccessToken(config *OAuthConfig, refreshToken string) (*TokenResponse, error) {
	params := url.Values{}
	params.Set("grant_type", "refresh_token")
	params.Set("refresh_token", refreshToken)
	params.Set("client_id", config.ClientID)
	params.Set("client_secret", config.ClientSecret)

	resp, err := http.PostForm(TokenURL, params)
	if err != nil {
		return nil, fmt.Errorf("刷新token失败: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %v", err)
	}

	var tokenResp TokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("解析响应失败: %v", err)
	}

	if tokenResp.Error != "" {
		return nil, fmt.Errorf("刷新token失败: %s - %s", tokenResp.Error, tokenResp.ErrorDescription)
	}

	return &tokenResp, nil
}

// 启动HTTP服务器接收授权回调
func startAuthServer(config *OAuthConfig, port int) (*TokenResponse, error) {
	authURL := generateAuthURL(config)
	logger.Info("请在浏览器中打开以下URL进行授权:\n%s", authURL)
	logger.Info("等待授权回调...")

	tokenChan := make(chan *TokenResponse, 1)
	errChan := make(chan error, 1)

	server := &http.Server{
		Addr: ":" + strconv.Itoa(port),
	}

	http.HandleFunc(CallbackPath, func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		errorParam := r.URL.Query().Get("error")

		if errorParam != "" {
			errDesc := r.URL.Query().Get("error_description")
			errChan <- fmt.Errorf("授权失败: %s - %s", errorParam, errDesc)
			fmt.Fprintf(w, "授权失败: %s", errorParam)
			return
		}

		if code == "" {
			errChan <- fmt.Errorf("未获取到授权码")
			fmt.Fprintf(w, "未获取到授权码")
			return
		}

		logger.Debug("收到授权码: %s", code)
		logger.Info("正在获取access_token...")

		tokenResp, err := getAccessToken(config, code)
		if err != nil {
			errChan <- err
			fmt.Fprintf(w, "获取access_token失败: %v", err)
			return
		}

		tokenChan <- tokenResp
		fmt.Fprintf(w, `
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>授权成功</title>
    <style>
        body { font-family: Arial, sans-serif; text-align: center; padding: 50px; }
        .success { color: #4CAF50; }
        .token { background: #f5f5f5; padding: 10px; margin: 20px 0; word-break: break-all; }
    </style>
</head>
<body>
    <h1 class="success">✅ 授权成功！</h1>
    <p>Access Token已获取，您可以关闭此页面。</p>
    <div class="token">
        <strong>Access Token:</strong><br>
        %s
    </div>
    <p><em>程序将自动保存token到配置文件中。</em></p>
</body>
</html>`, tokenResp.AccessToken)
	})

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- fmt.Errorf("启动服务器失败: %v", err)
		}
	}()

	// 等待授权结果或超时
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	select {
	case token := <-tokenChan:
		server.Shutdown(ctx)
		return token, nil
	case err := <-errChan:
		server.Shutdown(ctx)
		return nil, err
	case <-ctx.Done():
		server.Shutdown(ctx)
		return nil, fmt.Errorf("授权超时（5分钟）")
	}
}
