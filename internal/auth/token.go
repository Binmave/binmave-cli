package auth

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/Binmave/binmave-cli/internal/config"
)

// TokenInfo stores OAuth token information
type TokenInfo struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	TokenType    string    `json:"token_type"`
	ExpiresAt    time.Time `json:"expires_at"`
	Scope        string    `json:"scope"`
}

// IsExpired returns true if the access token is expired
func (t *TokenInfo) IsExpired() bool {
	// Consider expired 1 minute before actual expiry
	return time.Now().Add(time.Minute).After(t.ExpiresAt)
}

// IsValid returns true if the token exists and is not expired
func (t *TokenInfo) IsValid() bool {
	return t.AccessToken != "" && !t.IsExpired()
}

// LoadToken loads the stored token from disk
func LoadToken() (*TokenInfo, error) {
	path, err := getCredentialsPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var token TokenInfo
	if err := json.Unmarshal(data, &token); err != nil {
		return nil, err
	}

	return &token, nil
}

// SaveToken saves the token to disk
func SaveToken(token *TokenInfo) error {
	path, err := getCredentialsPath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(token, "", "  ")
	if err != nil {
		return err
	}

	// Write with restricted permissions (owner only)
	return os.WriteFile(path, data, 0600)
}

// DeleteToken removes the stored token
func DeleteToken() error {
	path, err := getCredentialsPath()
	if err != nil {
		return err
	}

	err = os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// getCredentialsPath returns the path to the credentials file
func getCredentialsPath() (string, error) {
	configDir, err := config.GetConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, config.CredentialsFile+".json"), nil
}

// GetValidToken returns a valid token, refreshing if necessary
func GetValidToken() (*TokenInfo, error) {
	token, err := LoadToken()
	if err != nil {
		return nil, err
	}

	if token == nil {
		return nil, nil
	}

	if token.IsValid() {
		return token, nil
	}

	// Token expired, try to refresh
	if token.RefreshToken != "" {
		newToken, err := RefreshAccessToken(token.RefreshToken)
		if err != nil {
			// Refresh failed, return nil to trigger new login
			return nil, nil
		}
		return newToken, nil
	}

	return nil, nil
}
