package http

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// oauthServer is a token endpoint (/token) plus a resource (/api) that echoes
// the bearer token it received.
type oauthServer struct {
	*httptest.Server
	tokenCalls atomic.Int32
	expiresIn  int
	// rejectFirst makes /api answer 401 to the first token it sees as "tok-1".
	rejectTok atomic.Value
}

func newOAuthServer(t *testing.T, expiresIn int) *oauthServer {
	s := &oauthServer{expiresIn: expiresIn}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			n := s.tokenCalls.Add(1)
			_ = r.ParseForm()
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintf(w, `{"access_token":"tok-%d","token_type":"Bearer","expires_in":%d}`, n, s.expiresIn)
		case "/api":
			auth := r.Header.Get("Authorization")
			if rej, _ := s.rejectTok.Load().(string); rej != "" && auth == "Bearer "+rej {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			_, _ = w.Write([]byte(auth))
		}
	}))
	t.Cleanup(s.Close)
	return s
}

func oauthReq(s *oauthServer) *Request {
	req := NewRequest("GET", s.URL+"/api")
	req.OAuth2Auth = &OAuth2AuthCredentials{
		TokenURL: s.URL + "/token", ClientID: "id", ClientSecret: "secret",
		Scopes: []string{"read"}, GrantType: "client_credentials",
	}
	return req
}

func TestClient_OAuth2TokenIsCached(t *testing.T) {
	s := newOAuthServer(t, 3600)
	client := NewClient()
	for i := 0; i < 5; i++ {
		resp, err := client.Do(oauthReq(s))
		require.NoError(t, err)
		assert.Equal(t, "Bearer tok-1", resp.BodyString())
	}
	assert.EqualValues(t, 1, s.tokenCalls.Load(), "one token POST for five requests")

	// Different scope, grant or user means a different token.
	other := oauthReq(s)
	other.OAuth2Auth.Scopes = []string{"write"}
	resp, err := client.Do(other)
	require.NoError(t, err)
	assert.Equal(t, "Bearer tok-2", resp.BodyString())

	pw := oauthReq(s)
	pw.OAuth2Auth.GrantType, pw.OAuth2Auth.Username, pw.OAuth2Auth.Password = "password", "alice", "pw"
	resp, err = client.Do(pw)
	require.NoError(t, err)
	assert.Equal(t, "Bearer tok-3", resp.BodyString())
	pw.OAuth2Auth.Username = "bob"
	resp, err = client.Do(pw)
	require.NoError(t, err)
	assert.Equal(t, "Bearer tok-4", resp.BodyString())
	assert.EqualValues(t, 4, s.tokenCalls.Load())
}

func TestClient_OAuth2ExpiredTokenIsRefetched(t *testing.T) {
	s := newOAuthServer(t, 10) // inside the 30s expiry skew: never reusable
	client := NewClient()
	for i := 1; i <= 2; i++ {
		resp, err := client.Do(oauthReq(s))
		require.NoError(t, err)
		assert.Equal(t, fmt.Sprintf("Bearer tok-%d", i), resp.BodyString())
	}
}

func TestClient_OAuth2ConcurrentRequestsShareOneTokenRequest(t *testing.T) {
	s := newOAuthServer(t, 3600)
	client := NewClient()
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp, err := client.Do(oauthReq(s))
			assert.NoError(t, err)
			if resp != nil {
				assert.Equal(t, "Bearer tok-1", resp.BodyString())
			}
		}()
	}
	wg.Wait()
	assert.EqualValues(t, 1, s.tokenCalls.Load())
}

func TestClient_OAuth2RevokedCachedTokenIsRefreshedOnce(t *testing.T) {
	s := newOAuthServer(t, 3600)
	client := NewClient()
	resp, err := client.Do(oauthReq(s))
	require.NoError(t, err)
	assert.Equal(t, "Bearer tok-1", resp.BodyString())

	s.rejectTok.Store("tok-1")
	resp, err = client.Do(oauthReq(s))
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, "Bearer tok-2", resp.BodyString())
	assert.EqualValues(t, 2, s.tokenCalls.Load())
}

func TestClient_OAuth2TokenRequestHonorsTimeout(t *testing.T) {
	release := make(chan struct{})
	hang := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	defer hang.Close()
	defer close(release)

	req := NewRequest("GET", hang.URL+"/api")
	req.OAuth2Auth = &OAuth2AuthCredentials{TokenURL: hang.URL + "/token", ClientID: "i", ClientSecret: "s", GrantType: "client_credentials"}

	done := make(chan error, 1)
	go func() {
		_, err := NewClient(WithTimeout(300 * time.Millisecond)).Do(req)
		done <- err
	}()
	select {
	case err := <-done:
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get OAuth2 token")
	case <-time.After(5 * time.Second):
		t.Fatal("token request ignored the request timeout and hung")
	}
}

func TestClient_OAuth2TokenFailureIsReported(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"invalid_client","error_description":"bad secret"}`))
	}))
	defer srv.Close()
	req := NewRequest("GET", srv.URL+"/api")
	req.OAuth2Auth = &OAuth2AuthCredentials{TokenURL: srv.URL + "/token", ClientID: "i", ClientSecret: "s", GrantType: "client_credentials"}
	_, err := NewClient().Do(req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid_client")
}
