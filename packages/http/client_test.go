package http

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_Get(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/test", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"message": "hello"}`))
	}))
	defer server.Close()

	client := NewClient()
	resp, err := client.Get(server.URL+"/test", nil)

	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header("Content-Type"))
	assert.Contains(t, resp.BodyString(), "hello")
}

func TestClient_Post(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id": 123}`))
	}))
	defer server.Close()

	client := NewClient()
	resp, err := client.Post(server.URL, `{"name": "test"}`, map[string]string{
		"Content-Type": "application/json",
	})

	require.NoError(t, err)
	assert.Equal(t, 201, resp.StatusCode)
	assert.Contains(t, resp.BodyString(), "123")
}

func TestClient_WithTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(WithTimeout(50 * time.Millisecond))
	_, err := client.Get(server.URL, nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context deadline exceeded")
}

func TestClient_WithDefaultHeader(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "test-token", r.Header.Get("Authorization"))
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(WithDefaultHeader("Authorization", "test-token"))
	resp, err := client.Get(server.URL, nil)

	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
}

func TestClient_WithDefaultHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "test-token", r.Header.Get("Authorization"))
		assert.Equal(t, "custom-agent", r.Header.Get("User-Agent"))
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(WithDefaultHeaders(map[string]string{
		"Authorization": "test-token",
		"User-Agent":    "custom-agent",
	}))
	resp, err := client.Get(server.URL, nil)

	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
}

func TestClient_FollowRedirects(t *testing.T) {
	redirectCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/final" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`final`))
			return
		}
		redirectCount++
		http.Redirect(w, r, "/final", http.StatusFound)
	}))
	defer server.Close()

	client := NewClient(WithFollowRedirects(true))
	resp, err := client.Get(server.URL+"/redirect", nil)

	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, "final", resp.BodyString())
	assert.Equal(t, 1, redirectCount)
}

func TestClient_NoFollowRedirects(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/final", http.StatusFound)
	}))
	defer server.Close()

	client := NewClient(WithFollowRedirects(false))
	resp, err := client.Get(server.URL+"/redirect", nil)

	require.NoError(t, err)
	assert.Equal(t, 302, resp.StatusCode)
}

func TestClient_MaxRedirects(t *testing.T) {
	redirectCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		redirectCount++
		// Infinite redirect loop
		http.Redirect(w, r, "/redirect", http.StatusFound)
	}))
	defer server.Close()

	client := NewClient(WithMaxRedirects(3))
	resp, err := client.Get(server.URL+"/redirect", nil)

	require.NoError(t, err)
	// Should stop after max redirects and return the redirect response
	assert.Equal(t, 302, resp.StatusCode)
	// The redirect check happens after following, so we get maxRedirects requests
	assert.LessOrEqual(t, redirectCount, 4)
}

func TestValidateURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid http URL",
			url:     "http://example.com/path",
			wantErr: false,
		},
		{
			name:    "valid https URL",
			url:     "https://example.com/path",
			wantErr: false,
		},
		{
			name:    "invalid scheme",
			url:     "ftp://example.com",
			wantErr: true,
			errMsg:  "unsupported URL scheme",
		},
		{
			name:    "missing scheme",
			url:     "example.com/path",
			wantErr: true,
			errMsg:  "unsupported URL scheme",
		},
		{
			name:    "file scheme",
			url:     "file:///etc/passwd",
			wantErr: true,
			errMsg:  "unsupported URL scheme",
		},
		{
			name:    "missing host",
			url:     "http:///path",
			wantErr: true,
			errMsg:  "URL must have a host",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateURL(tt.url)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidatePathWithinBase(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		baseDir string
		wantErr bool
	}{
		{
			name:    "path within base",
			path:    "/home/user/project/file.txt",
			baseDir: "/home/user/project",
			wantErr: false,
		},
		{
			name:    "path traversal attempt",
			path:    "/home/user/project/../../../etc/passwd",
			baseDir: "/home/user/project",
			wantErr: true,
		},
		{
			name:    "relative path traversal",
			path:    "../../../etc/passwd",
			baseDir: "/home/user/project",
			wantErr: true,
		},
		{
			name:    "empty base dir",
			path:    "/any/path",
			baseDir: "",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePathWithinBase(tt.path, tt.baseDir)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "path traversal")
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestResponse_IsSuccess(t *testing.T) {
	tests := []struct {
		statusCode int
		expected   bool
	}{
		{200, true},
		{201, true},
		{204, true},
		{299, true},
		{300, false},
		{400, false},
		{404, false},
		{500, false},
	}

	for _, tt := range tests {
		resp := &Response{StatusCode: tt.statusCode}
		assert.Equal(t, tt.expected, resp.IsSuccess(), "StatusCode: %d", tt.statusCode)
	}
}

func TestResponse_IsJSON(t *testing.T) {
	tests := []struct {
		contentType string
		expected    bool
	}{
		{"application/json", true},
		{"application/json; charset=utf-8", true},
		{"text/html", false},
		{"text/plain", false},
		{"", false},
	}

	for _, tt := range tests {
		resp := &Response{Headers: map[string]string{"Content-Type": tt.contentType}}
		assert.Equal(t, tt.expected, resp.IsJSON(), "Content-Type: %s", tt.contentType)
	}
}
