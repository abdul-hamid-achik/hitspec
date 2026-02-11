package oauth2

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Token.IsExpired
// ---------------------------------------------------------------------------

func TestToken_IsExpired_ZeroTime(t *testing.T) {
	tok := &Token{AccessToken: "abc"}
	if tok.IsExpired() {
		t.Error("token with zero ExpiresAt should not be considered expired")
	}
}

func TestToken_IsExpired_FarFuture(t *testing.T) {
	tok := &Token{
		AccessToken: "abc",
		ExpiresAt:   time.Now().Add(1 * time.Hour),
	}
	if tok.IsExpired() {
		t.Error("token expiring in 1 hour should not be expired")
	}
}

func TestToken_IsExpired_AlreadyPast(t *testing.T) {
	tok := &Token{
		AccessToken: "abc",
		ExpiresAt:   time.Now().Add(-1 * time.Minute),
	}
	if !tok.IsExpired() {
		t.Error("token that expired 1 minute ago should be expired")
	}
}

func TestToken_IsExpired_Within30sBuffer(t *testing.T) {
	// Expires in 20 seconds — inside the 30-second buffer, so treated as expired.
	tok := &Token{
		AccessToken: "abc",
		ExpiresAt:   time.Now().Add(20 * time.Second),
	}
	if !tok.IsExpired() {
		t.Error("token expiring in 20s should be considered expired (30s buffer)")
	}
}

func TestToken_IsExpired_JustOutsideBuffer(t *testing.T) {
	// Expires in 60 seconds — outside the 30-second buffer.
	tok := &Token{
		AccessToken: "abc",
		ExpiresAt:   time.Now().Add(60 * time.Second),
	}
	if tok.IsExpired() {
		t.Error("token expiring in 60s should not be considered expired")
	}
}

// ---------------------------------------------------------------------------
// NewProvider
// ---------------------------------------------------------------------------

func TestNewProvider(t *testing.T) {
	cfg := &Config{
		TokenURL:     "https://example.com/token",
		ClientID:     "cid",
		ClientSecret: "csecret",
		GrantType:    ClientCredentials,
	}
	p := NewProvider(cfg)
	if p == nil {
		t.Fatal("NewProvider returned nil")
	}
	if p.config != cfg {
		t.Error("provider should store the supplied config")
	}
	if p.httpClient == nil {
		t.Error("provider should have an http client")
	}
	if p.cache == nil {
		t.Error("provider should have a token cache")
	}
}

// ---------------------------------------------------------------------------
// helpers — mock OAuth2 token server
// ---------------------------------------------------------------------------

// mockTokenServer returns an httptest.Server that acts like an OAuth2 token
// endpoint.  The handler validates the request and returns a JSON token.
func mockTokenServer(t *testing.T, opts mockOpts) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		ct := r.Header.Get("Content-Type")
		if !strings.Contains(ct, "application/x-www-form-urlencoded") {
			t.Errorf("unexpected Content-Type: %s", ct)
		}

		// Validate Basic auth header if clientID/secret expected.
		if opts.expectBasicAuth {
			auth := r.Header.Get("Authorization")
			expected := "Basic " + base64.StdEncoding.EncodeToString([]byte(opts.clientID+":"+opts.clientSecret))
			if auth != expected {
				t.Errorf("wrong Authorization header: got %q, want %q", auth, expected)
			}
		}

		if err := r.ParseForm(); err != nil {
			t.Fatalf("ParseForm: %v", err)
		}

		if opts.validateForm != nil {
			opts.validateForm(t, r.Form)
		}

		// Return error response if requested.
		if opts.statusCode != 0 && opts.statusCode != http.StatusOK {
			w.WriteHeader(opts.statusCode)
			if opts.responseBody != "" {
				_, _ = w.Write([]byte(opts.responseBody))
			}
			return
		}

		tok := map[string]any{
			"access_token": opts.accessToken,
			"token_type":   "Bearer",
			"expires_in":   opts.expiresIn,
		}
		if opts.refreshToken != "" {
			tok["refresh_token"] = opts.refreshToken
		}
		if opts.scope != "" {
			tok["scope"] = opts.scope
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(tok)
	}))
}

type mockOpts struct {
	expectBasicAuth bool
	clientID        string
	clientSecret    string
	accessToken     string
	refreshToken    string
	scope           string
	expiresIn       int
	statusCode      int    // 0 means 200
	responseBody    string // used when statusCode != 200
	validateForm    func(t *testing.T, form url.Values)
}

// ---------------------------------------------------------------------------
// Provider.GetToken — client_credentials grant
// ---------------------------------------------------------------------------

func TestGetToken_ClientCredentials(t *testing.T) {
	srv := mockTokenServer(t, mockOpts{
		expectBasicAuth: true,
		clientID:        "myid",
		clientSecret:    "mysecret",
		accessToken:     "tok_cc_123",
		expiresIn:       3600,
		scope:           "read write",
		validateForm: func(t *testing.T, form url.Values) {
			t.Helper()
			if got := form.Get("grant_type"); got != "client_credentials" {
				t.Errorf("grant_type = %q, want client_credentials", got)
			}
			if got := form.Get("scope"); got != "read write" {
				t.Errorf("scope = %q, want %q", got, "read write")
			}
		},
	})
	defer srv.Close()

	cfg := &Config{
		TokenURL:     srv.URL,
		ClientID:     "myid",
		ClientSecret: "mysecret",
		Scopes:       []string{"read", "write"},
		GrantType:    ClientCredentials,
	}
	p := NewProvider(cfg)
	tok, err := p.GetToken()
	if err != nil {
		t.Fatalf("GetToken: %v", err)
	}
	if tok.AccessToken != "tok_cc_123" {
		t.Errorf("AccessToken = %q, want tok_cc_123", tok.AccessToken)
	}
	if tok.TokenType != "Bearer" {
		t.Errorf("TokenType = %q, want Bearer", tok.TokenType)
	}
	if tok.ExpiresIn != 3600 {
		t.Errorf("ExpiresIn = %d, want 3600", tok.ExpiresIn)
	}
	if tok.ExpiresAt.IsZero() {
		t.Error("ExpiresAt should be set when ExpiresIn > 0")
	}
}

// ---------------------------------------------------------------------------
// Provider.GetToken — password grant
// ---------------------------------------------------------------------------

func TestGetToken_PasswordGrant(t *testing.T) {
	srv := mockTokenServer(t, mockOpts{
		expectBasicAuth: true,
		clientID:        "cid",
		clientSecret:    "csec",
		accessToken:     "tok_pw_456",
		refreshToken:    "rt_789",
		expiresIn:       1800,
		validateForm: func(t *testing.T, form url.Values) {
			t.Helper()
			if got := form.Get("grant_type"); got != "password" {
				t.Errorf("grant_type = %q, want password", got)
			}
			if got := form.Get("username"); got != "alice" {
				t.Errorf("username = %q, want alice", got)
			}
			if got := form.Get("password"); got != "s3cret" {
				t.Errorf("password = %q, want s3cret", got)
			}
		},
	})
	defer srv.Close()

	cfg := &Config{
		TokenURL:     srv.URL,
		ClientID:     "cid",
		ClientSecret: "csec",
		Username:     "alice",
		Password:     "s3cret",
		GrantType:    Password,
	}
	p := NewProvider(cfg)
	tok, err := p.GetToken()
	if err != nil {
		t.Fatalf("GetToken: %v", err)
	}
	if tok.AccessToken != "tok_pw_456" {
		t.Errorf("AccessToken = %q, want tok_pw_456", tok.AccessToken)
	}
	if tok.RefreshToken != "rt_789" {
		t.Errorf("RefreshToken = %q, want rt_789", tok.RefreshToken)
	}
}

// ---------------------------------------------------------------------------
// Provider.GetToken — caching: returns cached token on second call
// ---------------------------------------------------------------------------

func TestGetToken_ReturnsCachedToken(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "cached_tok",
			"token_type":   "Bearer",
			"expires_in":   3600,
		})
	}))
	defer srv.Close()

	cfg := &Config{
		TokenURL:     srv.URL,
		ClientID:     "c",
		ClientSecret: "s",
		GrantType:    ClientCredentials,
	}
	p := NewProvider(cfg)

	tok1, err := p.GetToken()
	if err != nil {
		t.Fatalf("first GetToken: %v", err)
	}

	tok2, err := p.GetToken()
	if err != nil {
		t.Fatalf("second GetToken: %v", err)
	}

	if calls != 1 {
		t.Errorf("expected exactly 1 HTTP call, got %d", calls)
	}
	if tok1.AccessToken != tok2.AccessToken {
		t.Error("second call should return the same cached token")
	}
}

// ---------------------------------------------------------------------------
// Provider.GetToken — refreshes expired cached token
// ---------------------------------------------------------------------------

func TestGetToken_RefreshesExpiredCachedToken(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "tok_" + strings.Repeat("x", callCount),
			"token_type":   "Bearer",
			"expires_in":   3600,
		})
	}))
	defer srv.Close()

	cfg := &Config{
		TokenURL:     srv.URL,
		ClientID:     "c",
		ClientSecret: "s",
		GrantType:    ClientCredentials,
	}
	p := NewProvider(cfg)

	// First call — fetches from server.
	tok1, err := p.GetToken()
	if err != nil {
		t.Fatalf("first GetToken: %v", err)
	}
	if callCount != 1 {
		t.Fatalf("expected 1 call, got %d", callCount)
	}

	// Manually expire the cached token by modifying ExpiresAt.
	cacheKey := p.getCacheKey()
	cached := p.cache.Get(cacheKey)
	if cached == nil {
		t.Fatal("expected token to be cached")
	}
	cached.ExpiresAt = time.Now().Add(-1 * time.Minute) // expired

	// Second call — should fetch a new token because cached one is expired.
	tok2, err := p.GetToken()
	if err != nil {
		t.Fatalf("second GetToken: %v", err)
	}
	if callCount != 2 {
		t.Errorf("expected 2 calls after expiration, got %d", callCount)
	}
	if tok1.AccessToken == tok2.AccessToken {
		t.Error("second token should differ from the expired first token")
	}
}

// ---------------------------------------------------------------------------
// Provider.RefreshAccessToken
// ---------------------------------------------------------------------------

func TestRefreshAccessToken(t *testing.T) {
	srv := mockTokenServer(t, mockOpts{
		expectBasicAuth: true,
		clientID:        "cid",
		clientSecret:    "csec",
		accessToken:     "refreshed_tok",
		refreshToken:    "new_rt",
		expiresIn:       7200,
		validateForm: func(t *testing.T, form url.Values) {
			t.Helper()
			if got := form.Get("grant_type"); got != "refresh_token" {
				t.Errorf("grant_type = %q, want refresh_token", got)
			}
			if got := form.Get("refresh_token"); got != "old_rt" {
				t.Errorf("refresh_token = %q, want old_rt", got)
			}
		},
	})
	defer srv.Close()

	cfg := &Config{
		TokenURL:     srv.URL,
		ClientID:     "cid",
		ClientSecret: "csec",
		GrantType:    ClientCredentials,
	}
	p := NewProvider(cfg)

	tok, err := p.RefreshAccessToken("old_rt")
	if err != nil {
		t.Fatalf("RefreshAccessToken: %v", err)
	}
	if tok.AccessToken != "refreshed_tok" {
		t.Errorf("AccessToken = %q, want refreshed_tok", tok.AccessToken)
	}
	if tok.ExpiresIn != 7200 {
		t.Errorf("ExpiresIn = %d, want 7200", tok.ExpiresIn)
	}
}

// ---------------------------------------------------------------------------
// ParseAuthAnnotation
// ---------------------------------------------------------------------------

func TestParseAuthAnnotation_ClientCredentials_MinimalParams(t *testing.T) {
	cfg, err := ParseAuthAnnotation([]string{"client_credentials", "https://example.com/token", "cid", "csec"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.GrantType != ClientCredentials {
		t.Errorf("GrantType = %q, want client_credentials", cfg.GrantType)
	}
	if cfg.TokenURL != "https://example.com/token" {
		t.Errorf("TokenURL = %q", cfg.TokenURL)
	}
	if cfg.ClientID != "cid" {
		t.Errorf("ClientID = %q", cfg.ClientID)
	}
	if cfg.ClientSecret != "csec" {
		t.Errorf("ClientSecret = %q", cfg.ClientSecret)
	}
	if len(cfg.Scopes) != 0 {
		t.Errorf("Scopes should be empty, got %v", cfg.Scopes)
	}
}

func TestParseAuthAnnotation_ClientCredentials_WithScopes(t *testing.T) {
	cfg, err := ParseAuthAnnotation([]string{"client_credentials", "https://example.com/token", "cid", "csec", "read,write,admin"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Scopes) != 3 {
		t.Fatalf("expected 3 scopes, got %d: %v", len(cfg.Scopes), cfg.Scopes)
	}
	expected := []string{"read", "write", "admin"}
	for i, s := range cfg.Scopes {
		if s != expected[i] {
			t.Errorf("Scopes[%d] = %q, want %q", i, s, expected[i])
		}
	}
}

func TestParseAuthAnnotation_PasswordGrant(t *testing.T) {
	cfg, err := ParseAuthAnnotation([]string{"password", "https://example.com/token", "cid", "csec", "alice", "pass123"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.GrantType != Password {
		t.Errorf("GrantType = %q, want password", cfg.GrantType)
	}
	if cfg.Username != "alice" {
		t.Errorf("Username = %q, want alice", cfg.Username)
	}
	if cfg.Password != "pass123" {
		t.Errorf("Password = %q, want pass123", cfg.Password)
	}
}

func TestParseAuthAnnotation_PasswordGrant_WithScopes(t *testing.T) {
	cfg, err := ParseAuthAnnotation([]string{"password", "https://example.com/token", "cid", "csec", "alice", "pass123", "scope1,scope2"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Scopes) != 2 {
		t.Fatalf("expected 2 scopes, got %d", len(cfg.Scopes))
	}
}

func TestParseAuthAnnotation_TooFewParams(t *testing.T) {
	_, err := ParseAuthAnnotation([]string{"client_credentials", "https://example.com/token"})
	if err == nil {
		t.Error("expected error for too few params")
	}
	if !strings.Contains(err.Error(), "at least") {
		t.Errorf("error message should mention minimum params: %v", err)
	}
}

func TestParseAuthAnnotation_PasswordGrant_MissingUserPass(t *testing.T) {
	_, err := ParseAuthAnnotation([]string{"password", "https://example.com/token", "cid", "csec", "onlyuser"})
	if err == nil {
		t.Error("expected error when password grant missing username/password")
	}
	if !strings.Contains(err.Error(), "password grant") {
		t.Errorf("error message should mention password grant: %v", err)
	}
}

func TestParseAuthAnnotation_UnsupportedGrant(t *testing.T) {
	_, err := ParseAuthAnnotation([]string{"implicit", "https://example.com/token", "cid", "csec"})
	if err == nil {
		t.Error("expected error for unsupported grant type")
	}
	if !strings.Contains(err.Error(), "unsupported") {
		t.Errorf("error message should mention unsupported: %v", err)
	}
}

// ---------------------------------------------------------------------------
// TokenCache — Get, Set, Delete, Clear
// ---------------------------------------------------------------------------

func TestTokenCache_GetSet(t *testing.T) {
	c := NewTokenCache()
	tok := &Token{AccessToken: "a"}
	c.Set("k1", tok)

	got := c.Get("k1")
	if got == nil {
		t.Fatal("Get returned nil for existing key")
	}
	if got.AccessToken != "a" {
		t.Errorf("AccessToken = %q, want a", got.AccessToken)
	}
}

func TestTokenCache_GetMiss(t *testing.T) {
	c := NewTokenCache()
	if got := c.Get("nonexistent"); got != nil {
		t.Errorf("Get for missing key should return nil, got %v", got)
	}
}

func TestTokenCache_Delete(t *testing.T) {
	c := NewTokenCache()
	c.Set("k1", &Token{AccessToken: "a"})
	c.Delete("k1")
	if got := c.Get("k1"); got != nil {
		t.Error("expected nil after Delete")
	}
}

func TestTokenCache_DeleteNonexistent(t *testing.T) {
	c := NewTokenCache()
	// Should not panic.
	c.Delete("nope")
}

func TestTokenCache_Clear(t *testing.T) {
	c := NewTokenCache()
	c.Set("k1", &Token{AccessToken: "a"})
	c.Set("k2", &Token{AccessToken: "b"})
	c.Clear()
	if c.Get("k1") != nil || c.Get("k2") != nil {
		t.Error("Clear should remove all entries")
	}
}

func TestTokenCache_Concurrency(t *testing.T) {
	c := NewTokenCache()
	var wg sync.WaitGroup
	const goroutines = 50

	// Concurrent writes
	for i := range goroutines {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			key := "key"
			c.Set(key, &Token{AccessToken: strings.Repeat("x", idx)})
			_ = c.Get(key)
		}(i)
	}
	wg.Wait()

	// If we got here without a race detector complaint, concurrency is fine.
	got := c.Get("key")
	if got == nil {
		t.Error("expected a token to be stored after concurrent writes")
	}
}

// ---------------------------------------------------------------------------
// Error cases
// ---------------------------------------------------------------------------

func TestGetToken_Non200Response_WithErrorBody(t *testing.T) {
	srv := mockTokenServer(t, mockOpts{
		statusCode:   http.StatusUnauthorized,
		responseBody: `{"error":"invalid_client","error_description":"bad credentials"}`,
	})
	defer srv.Close()

	cfg := &Config{
		TokenURL:  srv.URL,
		ClientID:  "bad",
		GrantType: ClientCredentials,
	}
	p := NewProvider(cfg)
	_, err := p.GetToken()
	if err == nil {
		t.Fatal("expected error for 401 response")
	}
	if !strings.Contains(err.Error(), "invalid_client") {
		t.Errorf("error should contain OAuth error code: %v", err)
	}
	if !strings.Contains(err.Error(), "bad credentials") {
		t.Errorf("error should contain error_description: %v", err)
	}
}

func TestGetToken_Non200Response_PlainBody(t *testing.T) {
	srv := mockTokenServer(t, mockOpts{
		statusCode:   http.StatusInternalServerError,
		responseBody: "internal server error",
	})
	defer srv.Close()

	cfg := &Config{
		TokenURL:  srv.URL,
		ClientID:  "c",
		GrantType: ClientCredentials,
	}
	p := NewProvider(cfg)
	_, err := p.GetToken()
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("error should contain status code: %v", err)
	}
}

func TestGetToken_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{not valid json`))
	}))
	defer srv.Close()

	cfg := &Config{
		TokenURL:  srv.URL,
		ClientID:  "c",
		GrantType: ClientCredentials,
	}
	p := NewProvider(cfg)
	_, err := p.GetToken()
	if err == nil {
		t.Fatal("expected error for invalid JSON response")
	}
	if !strings.Contains(err.Error(), "parse token response") {
		t.Errorf("error should mention parsing: %v", err)
	}
}

func TestGetToken_InvalidURL(t *testing.T) {
	cfg := &Config{
		TokenURL:  "http://127.0.0.1:1", // nothing listening
		ClientID:  "c",
		GrantType: ClientCredentials,
	}
	p := NewProvider(cfg)
	_, err := p.GetToken()
	if err == nil {
		t.Fatal("expected error for unreachable server")
	}
	if !strings.Contains(err.Error(), "token request failed") {
		t.Errorf("error should mention request failure: %v", err)
	}
}

func TestGetToken_NoExpiresIn(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "no_expiry",
			"token_type":   "Bearer",
		})
	}))
	defer srv.Close()

	cfg := &Config{
		TokenURL:  srv.URL,
		ClientID:  "c",
		GrantType: ClientCredentials,
	}
	p := NewProvider(cfg)
	tok, err := p.GetToken()
	if err != nil {
		t.Fatalf("GetToken: %v", err)
	}
	if !tok.ExpiresAt.IsZero() {
		t.Error("ExpiresAt should be zero when expires_in is missing/0")
	}
	// Token with zero ExpiresAt is never expired.
	if tok.IsExpired() {
		t.Error("token without expiry should not be considered expired")
	}
}

// ---------------------------------------------------------------------------
// GetToken — default grant type falls back to client_credentials
// ---------------------------------------------------------------------------

func TestGetToken_DefaultGrantType(t *testing.T) {
	srv := mockTokenServer(t, mockOpts{
		accessToken: "default_tok",
		expiresIn:   3600,
		validateForm: func(t *testing.T, form url.Values) {
			t.Helper()
			if got := form.Get("grant_type"); got != "client_credentials" {
				t.Errorf("default grant_type should be client_credentials, got %q", got)
			}
		},
	})
	defer srv.Close()

	cfg := &Config{
		TokenURL:  srv.URL,
		GrantType: "", // empty — should default to client_credentials
	}
	p := NewProvider(cfg)
	tok, err := p.GetToken()
	if err != nil {
		t.Fatalf("GetToken: %v", err)
	}
	if tok.AccessToken != "default_tok" {
		t.Errorf("AccessToken = %q, want default_tok", tok.AccessToken)
	}
}

// ---------------------------------------------------------------------------
// GetToken — no basic auth when client ID/secret empty
// ---------------------------------------------------------------------------

func TestGetToken_NoBasicAuthWhenCredsEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if auth := r.Header.Get("Authorization"); auth != "" {
			t.Errorf("expected no Authorization header, got %q", auth)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "noauth_tok",
			"token_type":   "Bearer",
			"expires_in":   3600,
		})
	}))
	defer srv.Close()

	cfg := &Config{
		TokenURL:  srv.URL,
		GrantType: ClientCredentials,
	}
	p := NewProvider(cfg)
	tok, err := p.GetToken()
	if err != nil {
		t.Fatalf("GetToken: %v", err)
	}
	if tok.AccessToken != "noauth_tok" {
		t.Errorf("AccessToken = %q, want noauth_tok", tok.AccessToken)
	}
}

// ---------------------------------------------------------------------------
// GlobalCache
// ---------------------------------------------------------------------------

func TestGlobalCache_Exists(t *testing.T) {
	if GlobalCache == nil {
		t.Fatal("GlobalCache should be initialized")
	}
	// Basic smoke test.
	GlobalCache.Set("global_test", &Token{AccessToken: "g"})
	defer GlobalCache.Delete("global_test")
	tok := GlobalCache.Get("global_test")
	if tok == nil || tok.AccessToken != "g" {
		t.Error("GlobalCache Set/Get round-trip failed")
	}
}

// ---------------------------------------------------------------------------
// Provider.GetToken — client_credentials without scopes
// ---------------------------------------------------------------------------

func TestGetToken_ClientCredentials_NoScopes(t *testing.T) {
	srv := mockTokenServer(t, mockOpts{
		expectBasicAuth: true,
		clientID:        "cid",
		clientSecret:    "csec",
		accessToken:     "tok_noscope",
		expiresIn:       600,
		validateForm: func(t *testing.T, form url.Values) {
			t.Helper()
			if got := form.Get("scope"); got != "" {
				t.Errorf("scope should be absent, got %q", got)
			}
		},
	})
	defer srv.Close()

	cfg := &Config{
		TokenURL:     srv.URL,
		ClientID:     "cid",
		ClientSecret: "csec",
		GrantType:    ClientCredentials,
	}
	p := NewProvider(cfg)
	tok, err := p.GetToken()
	if err != nil {
		t.Fatalf("GetToken: %v", err)
	}
	if tok.AccessToken != "tok_noscope" {
		t.Errorf("AccessToken = %q, want tok_noscope", tok.AccessToken)
	}
}

// ---------------------------------------------------------------------------
// Provider.GetToken — password grant without scopes
// ---------------------------------------------------------------------------

func TestGetToken_PasswordGrant_NoScopes(t *testing.T) {
	srv := mockTokenServer(t, mockOpts{
		expectBasicAuth: true,
		clientID:        "cid",
		clientSecret:    "csec",
		accessToken:     "tok_pw_noscope",
		expiresIn:       600,
		validateForm: func(t *testing.T, form url.Values) {
			t.Helper()
			if got := form.Get("scope"); got != "" {
				t.Errorf("scope should be absent, got %q", got)
			}
			if got := form.Get("username"); got != "bob" {
				t.Errorf("username = %q, want bob", got)
			}
		},
	})
	defer srv.Close()

	cfg := &Config{
		TokenURL:     srv.URL,
		ClientID:     "cid",
		ClientSecret: "csec",
		Username:     "bob",
		Password:     "pw",
		GrantType:    Password,
	}
	p := NewProvider(cfg)
	tok, err := p.GetToken()
	if err != nil {
		t.Fatalf("GetToken: %v", err)
	}
	if tok.AccessToken != "tok_pw_noscope" {
		t.Errorf("AccessToken = %q", tok.AccessToken)
	}
}
