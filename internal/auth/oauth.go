package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/Binmave/binmave-cli/internal/config"
)

const (
	ClientID     = "cli"
	CallbackPort = 8765
	CallbackPath = "/callback"
	Scopes       = "openid profile IdentityServerApi offline_access"
)

// LoginResult contains the result of a login attempt
type LoginResult struct {
	Token *TokenInfo
	Error error
}

// Login performs the OAuth2 authorization code flow with PKCE
func Login(ctx context.Context) (*TokenInfo, error) {
	// Generate PKCE code verifier and challenge
	verifier, challenge, err := generatePKCE()
	if err != nil {
		return nil, fmt.Errorf("failed to generate PKCE: %w", err)
	}

	// Generate state for CSRF protection
	state, err := generateRandomString(32)
	if err != nil {
		return nil, fmt.Errorf("failed to generate state: %w", err)
	}

	// Channel to receive the authorization code
	codeChan := make(chan string, 1)
	errChan := make(chan error, 1)

	// Start local HTTP server to receive the callback
	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", CallbackPort))
	if err != nil {
		return nil, fmt.Errorf("failed to start callback server: %w", err)
	}

	server := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != CallbackPath {
				http.NotFound(w, r)
				return
			}

			// Check state
			if r.URL.Query().Get("state") != state {
				errChan <- fmt.Errorf("state mismatch")
				http.Error(w, "State mismatch", http.StatusBadRequest)
				return
			}

			// Check for error
			if errMsg := r.URL.Query().Get("error"); errMsg != "" {
				errDesc := r.URL.Query().Get("error_description")
				errChan <- fmt.Errorf("auth error: %s - %s", errMsg, errDesc)
				fmt.Fprintf(w, "<html><body><h1>Authentication Failed</h1><p>%s: %s</p><p>You can close this window.</p></body></html>", errMsg, errDesc)
				return
			}

			// Get authorization code
			code := r.URL.Query().Get("code")
			if code == "" {
				errChan <- fmt.Errorf("no authorization code received")
				http.Error(w, "No code received", http.StatusBadRequest)
				return
			}

			codeChan <- code
			fmt.Fprintf(w, "<html><body><h1>Authentication Successful!</h1><p>You can close this window and return to the CLI.</p></body></html>")
		}),
	}

	go func() {
		if err := server.Serve(listener); err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		server.Shutdown(shutdownCtx)
	}()

	// Build authorization URL
	authURL := buildAuthURL(state, challenge)

	// Open browser
	fmt.Println("Opening browser for authentication...")
	fmt.Printf("If the browser doesn't open, visit:\n%s\n\n", authURL)

	if err := openBrowser(authURL); err != nil {
		fmt.Printf("Failed to open browser: %v\n", err)
	}

	fmt.Println("Waiting for authentication...")

	// Wait for callback or timeout
	select {
	case code := <-codeChan:
		return exchangeCodeForToken(code, verifier)
	case err := <-errChan:
		return nil, err
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(5 * time.Minute):
		return nil, fmt.Errorf("authentication timed out")
	}
}

// RefreshAccessToken uses the refresh token to get a new access token
func RefreshAccessToken(refreshToken string) (*TokenInfo, error) {
	server := config.GetServer()
	tokenURL := fmt.Sprintf("%s/connect/token", server)

	data := url.Values{
		"grant_type":    {"refresh_token"},
		"client_id":     {ClientID},
		"refresh_token": {refreshToken},
	}

	resp, err := http.PostForm(tokenURL, data)
	if err != nil {
		return nil, fmt.Errorf("failed to refresh token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token refresh failed with status: %d", resp.StatusCode)
	}

	var tokenResp tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("failed to decode token response: %w", err)
	}

	token := &TokenInfo{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		TokenType:    tokenResp.TokenType,
		ExpiresAt:    time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second),
		Scope:        tokenResp.Scope,
	}

	// Save the new token
	if err := SaveToken(token); err != nil {
		return nil, fmt.Errorf("failed to save token: %w", err)
	}

	return token, nil
}

// tokenResponse represents the OAuth2 token response
type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	Scope        string `json:"scope"`
}

// buildAuthURL constructs the authorization URL
func buildAuthURL(state, codeChallenge string) string {
	server := config.GetServer()
	authEndpoint := fmt.Sprintf("%s/connect/authorize", server)

	params := url.Values{
		"client_id":             {ClientID},
		"response_type":         {"code"},
		"scope":                 {Scopes},
		"redirect_uri":          {fmt.Sprintf("http://localhost:%d%s", CallbackPort, CallbackPath)},
		"state":                 {state},
		"code_challenge":        {codeChallenge},
		"code_challenge_method": {"S256"},
	}

	return fmt.Sprintf("%s?%s", authEndpoint, params.Encode())
}

// exchangeCodeForToken exchanges the authorization code for tokens
func exchangeCodeForToken(code, verifier string) (*TokenInfo, error) {
	server := config.GetServer()
	tokenURL := fmt.Sprintf("%s/connect/token", server)

	data := url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {ClientID},
		"code":          {code},
		"redirect_uri":  {fmt.Sprintf("http://localhost:%d%s", CallbackPort, CallbackPath)},
		"code_verifier": {verifier},
	}

	resp, err := http.PostForm(tokenURL, data)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange code: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token exchange failed with status: %d", resp.StatusCode)
	}

	var tokenResp tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("failed to decode token response: %w", err)
	}

	token := &TokenInfo{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		TokenType:    tokenResp.TokenType,
		ExpiresAt:    time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second),
		Scope:        tokenResp.Scope,
	}

	// Save the token
	if err := SaveToken(token); err != nil {
		return nil, fmt.Errorf("failed to save token: %w", err)
	}

	return token, nil
}

// generatePKCE generates a PKCE code verifier and challenge
func generatePKCE() (verifier, challenge string, err error) {
	verifier, err = generateRandomString(64)
	if err != nil {
		return "", "", err
	}

	// Create S256 challenge
	h := sha256.Sum256([]byte(verifier))
	challenge = base64.RawURLEncoding.EncodeToString(h[:])

	return verifier, challenge, nil
}

// generateRandomString generates a cryptographically secure random string
func generateRandomString(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(bytes)[:length], nil
}

// openBrowser opens the default browser to the specified URL
func openBrowser(url string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default: // Linux and others
		cmd = exec.Command("xdg-open", url)
	}

	return cmd.Start()
}

// GetUserInfo fetches the current user's info from the userinfo endpoint
func GetUserInfo(token *TokenInfo) (*UserInfo, error) {
	server := config.GetServer()
	userInfoURL := fmt.Sprintf("%s/connect/userinfo", server)

	req, err := http.NewRequest("GET", userInfoURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token.AccessToken))

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch user info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("userinfo request failed with status: %d", resp.StatusCode)
	}

	var userInfo UserInfo
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		return nil, fmt.Errorf("failed to decode user info: %w", err)
	}

	return &userInfo, nil
}

// UserInfo represents the user info from the OIDC endpoint
type UserInfo struct {
	Sub               string   `json:"sub"`
	Name              string   `json:"name"`
	PreferredUsername string   `json:"preferred_username"`
	Email             string   `json:"email"`
	EmailVerified     bool     `json:"email_verified"`
	Roles             []string `json:"role"` // Can be array of roles
}

// Role returns the primary role (first one if multiple)
func (u *UserInfo) Role() string {
	if len(u.Roles) > 0 {
		return u.Roles[0]
	}
	return ""
}

// GetDisplayName returns the best display name for the user
func (u *UserInfo) GetDisplayName() string {
	if u.Name != "" {
		return u.Name
	}
	if u.PreferredUsername != "" {
		return u.PreferredUsername
	}
	if u.Email != "" {
		return strings.Split(u.Email, "@")[0]
	}
	return u.Sub
}
