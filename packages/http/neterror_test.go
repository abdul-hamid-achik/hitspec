package http

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// closedPortURL returns a URL on a loopback port nothing listens on.
func closedPortURL(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := l.Addr().String()
	require.NoError(t, l.Close())
	return "http://" + addr + "/"
}

func doErr(t *testing.T, c *Client, url string) error {
	t.Helper()
	_, err := c.Do(NewRequest("GET", url))
	require.Error(t, err)
	return err
}

func TestIsNetworkError_Nil(t *testing.T) {
	assert.False(t, IsNetworkError(nil))
}

func TestIsNetworkError_ConnectionRefused(t *testing.T) {
	assert.True(t, IsNetworkError(doErr(t, NewClient(), closedPortURL(t))))
}

func TestIsNetworkError_DNS(t *testing.T) {
	// .invalid never resolves (RFC 2606).
	err := doErr(t, NewClient(WithTimeout(10*time.Second)), "http://hitspec-nope.invalid/")
	assert.True(t, IsNetworkError(err), "got %v", err)
}

func TestIsNetworkError_ConnectionClosedBeforeResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, _, err := w.(http.Hijacker).Hijack()
		if err == nil {
			_ = conn.Close()
		}
	}))
	defer srv.Close()
	err := doErr(t, NewClient(WithTimeout(5*time.Second)), srv.URL)
	assert.True(t, IsNetworkError(err), "got %v", err)
}

func TestIsNetworkError_TLS(t *testing.T) {
	t.Run("untrusted certificate", func(t *testing.T) {
		srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
		defer srv.Close()
		err := doErr(t, NewClient(WithTimeout(5*time.Second)), srv.URL)
		assert.True(t, IsNetworkError(err), "got %v", err)
	})
	t.Run("https sent to a plain HTTP server", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
		defer srv.Close()
		err := doErr(t, NewClient(WithTimeout(5*time.Second)), "https://"+srv.Listener.Addr().String())
		assert.True(t, IsNetworkError(err), "got %v", err)
	})
}

func TestIsNetworkError_OAuth2TokenEndpointUnreachable(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer api.Close()
	req := NewRequest("GET", api.URL)
	req.OAuth2Auth = &OAuth2AuthCredentials{
		TokenURL: closedPortURL(t) + "token", ClientID: "id", ClientSecret: "s", GrantType: "client_credentials",
	}
	_, err := NewClient().Do(req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "OAuth2 token")
	assert.True(t, IsNetworkError(err), "got %v", err)
}

func TestIsNetworkError_OAuth2TokenEndpointRejects(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusUnauthorized)
	}))
	defer srv.Close()
	req := NewRequest("GET", srv.URL)
	req.OAuth2Auth = &OAuth2AuthCredentials{
		TokenURL: srv.URL + "/token", ClientID: "id", ClientSecret: "s", GrantType: "client_credentials",
	}
	_, err := NewClient().Do(req)
	require.Error(t, err)
	assert.False(t, IsNetworkError(err), "an answered token request is not a network error: %v", err)
}

func TestIsNetworkError_ResponseTimeoutIsNotNetwork(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	defer srv.Close()
	defer close(release)
	err := doErr(t, NewClient(WithTimeout(150*time.Millisecond)), srv.URL)
	assert.False(t, IsNetworkError(err), "got %v", err)
}

// A deadline that expires while the connection is still being set up (here a
// TLS handshake that never completes) means the host was never reached.
func TestIsNetworkError_TimeoutBeforeConnectedIsNetwork(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer l.Close()
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			defer c.Close() // held open, never answers the ClientHello
		}
	}()
	err = doErr(t, NewClient(WithTimeout(200*time.Millisecond)), "https://"+l.Addr().String())
	var ct *ConnectTimeoutError
	require.ErrorAs(t, err, &ct)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
	assert.True(t, IsNetworkError(err), "got %v", err)
}

func TestIsNetworkError_CancelledIsNotNetwork(t *testing.T) {
	// Cancelled while dialling must not read as a connection failure.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := NewClient().DoWithContext(ctx, NewRequest("GET", closedPortURL(t)))
	require.Error(t, err)
	assert.False(t, IsNetworkError(err), "got %v", err)
}

func TestIsNetworkError_BodyBreakingMidResponseIsNotNetwork(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "1000")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("0123456789"))
		w.(http.Flusher).Flush()
		conn, _, err := w.(http.Hijacker).Hijack()
		if err == nil {
			_ = conn.Close()
		}
	}))
	defer srv.Close()
	err := doErr(t, NewClient(WithTimeout(5*time.Second)), srv.URL)
	var bodyErr *BodyReadError
	require.ErrorAs(t, err, &bodyErr)
	assert.False(t, IsNetworkError(err), "a response arrived: %v", err)
}

func TestIsNetworkError_OtherErrors(t *testing.T) {
	assert.False(t, IsNetworkError(errors.New("expected status 200")))
	assert.False(t, IsNetworkError(io.EOF), "a bare EOF is not enough")
	assert.False(t, IsNetworkError(context.DeadlineExceeded))
	assert.False(t, IsNetworkError(&ResponseTooLargeError{Limit: 1}))
	assert.False(t, IsNetworkError(doErr(t, NewClient(), "ftp://example.com/")), "invalid scheme")
}
