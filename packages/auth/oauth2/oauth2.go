// Package oauth2 provides OAuth2 authentication support for hitspec.
package oauth2

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// GrantType represents the OAuth2 grant type
type GrantType string

const (
	// ClientCredentials is the client_credentials grant type
	ClientCredentials GrantType = "client_credentials"
	// Password is the password (resource owner) grant type
	Password GrantType = "password"
	// RefreshToken is the refresh_token grant type
	RefreshToken GrantType = "refresh_token"
)

// Config holds OAuth2 configuration
type Config struct {
	TokenURL     string
	ClientID     string
	ClientSecret string
	Scopes       []string
	Username     string // For password grant
	Password     string // For password grant
	GrantType    GrantType
}

// Token represents an OAuth2 access token
type Token struct {
	AccessToken  string    `json:"access_token"`
	TokenType    string    `json:"token_type"`
	ExpiresIn    int       `json:"expires_in"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	Scope        string    `json:"scope,omitempty"`
	ExpiresAt    time.Time `json:"-"`
}

// IsExpired checks if the token is expired
func (t *Token) IsExpired() bool {
	if t.ExpiresAt.IsZero() {
		return false
	}
	// Add a small buffer (30 seconds) to account for clock skew
	return time.Now().Add(30 * time.Second).After(t.ExpiresAt)
}

// Provider handles OAuth2 token acquisition
type Provider struct {
	config     *Config
	httpClient *http.Client
	cache      *TokenCache
}

// NewProvider creates a new OAuth2 provider
func NewProvider(config *Config) *Provider {
	return &Provider{
		config: config,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		cache: NewTokenCache(),
	}
}

// GetToken retrieves a valid access token, fetching a new one if necessary
func (p *Provider) GetToken() (*Token, error) {
	// Check cache first
	cacheKey := p.getCacheKey()
	if token := p.cache.Get(cacheKey); token != nil && !token.IsExpired() {
		return token, nil
	}

	// Fetch new token
	token, err := p.fetchToken()
	if err != nil {
		return nil, err
	}

	// Cache the token
	p.cache.Set(cacheKey, token)

	return token, nil
}

func (p *Provider) getCacheKey() string {
	return fmt.Sprintf("%s:%s:%s", p.config.TokenURL, p.config.ClientID, strings.Join(p.config.Scopes, ","))
}

func (p *Provider) fetchToken() (*Token, error) {
	switch p.config.GrantType {
	case ClientCredentials:
		return p.fetchClientCredentialsToken()
	case Password:
		return p.fetchPasswordToken()
	default:
		return p.fetchClientCredentialsToken()
	}
}

func (p *Provider) fetchClientCredentialsToken() (*Token, error) {
	data := url.Values{}
	data.Set("grant_type", "client_credentials")
	if len(p.config.Scopes) > 0 {
		data.Set("scope", strings.Join(p.config.Scopes, " "))
	}

	return p.doTokenRequest(data)
}

func (p *Provider) fetchPasswordToken() (*Token, error) {
	data := url.Values{}
	data.Set("grant_type", "password")
	data.Set("username", p.config.Username)
	data.Set("password", p.config.Password)
	if len(p.config.Scopes) > 0 {
		data.Set("scope", strings.Join(p.config.Scopes, " "))
	}

	return p.doTokenRequest(data)
}

func (p *Provider) doTokenRequest(data url.Values) (*Token, error) {
	req, err := http.NewRequest("POST", p.config.TokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create token request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Add client authentication
	if p.config.ClientID != "" && p.config.ClientSecret != "" {
		auth := base64.StdEncoding.EncodeToString([]byte(p.config.ClientID + ":" + p.config.ClientSecret))
		req.Header.Set("Authorization", "Basic "+auth)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("token request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read token response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		// Try to parse error response
		var errResp struct {
			Error            string `json:"error"`
			ErrorDescription string `json:"error_description"`
		}
		if json.Unmarshal(body, &errResp) == nil && errResp.Error != "" {
			return nil, fmt.Errorf("token request failed: %s - %s", errResp.Error, errResp.ErrorDescription)
		}
		return nil, fmt.Errorf("token request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var token Token
	if err := json.Unmarshal(body, &token); err != nil {
		return nil, fmt.Errorf("failed to parse token response: %w", err)
	}

	// Calculate expiration time
	if token.ExpiresIn > 0 {
		token.ExpiresAt = time.Now().Add(time.Duration(token.ExpiresIn) * time.Second)
	}

	return &token, nil
}

// RefreshAccessToken refreshes the access token using a refresh token
func (p *Provider) RefreshAccessToken(refreshToken string) (*Token, error) {
	data := url.Values{}
	data.Set("grant_type", "refresh_token")
	data.Set("refresh_token", refreshToken)

	return p.doTokenRequest(data)
}

// ParseAuthAnnotation parses OAuth2 auth annotation and returns config
// Format: oauth2 client_credentials tokenUrl clientId clientSecret [scope1,scope2]
// Or: oauth2 password tokenUrl clientId clientSecret username password [scope1,scope2]
func ParseAuthAnnotation(params []string) (*Config, error) {
	if len(params) < 4 {
		return nil, fmt.Errorf("oauth2 auth requires at least: grant_type tokenUrl clientId clientSecret")
	}

	config := &Config{
		GrantType:    GrantType(params[0]),
		TokenURL:     params[1],
		ClientID:     params[2],
		ClientSecret: params[3],
	}

	switch config.GrantType {
	case ClientCredentials:
		// Optional scopes at position 4
		if len(params) > 4 {
			config.Scopes = strings.Split(params[4], ",")
		}
	case Password:
		// Requires username and password at positions 4 and 5
		if len(params) < 6 {
			return nil, fmt.Errorf("oauth2 password grant requires: tokenUrl clientId clientSecret username password [scopes]")
		}
		config.Username = params[4]
		config.Password = params[5]
		if len(params) > 6 {
			config.Scopes = strings.Split(params[6], ",")
		}
	default:
		return nil, fmt.Errorf("unsupported OAuth2 grant type: %s", config.GrantType)
	}

	return config, nil
}
