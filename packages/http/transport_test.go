package http

import (
	"compress/gzip"
	"compress/zlib"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// clearProxyEnv unsets every proxy variable so a developer's shell cannot
// change a test's outcome.
func clearProxyEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{"HTTP_PROXY", "http_proxy", "HTTPS_PROXY", "https_proxy", "NO_PROXY", "no_proxy", "REQUEST_METHOD"} {
		t.Setenv(k, "")
	}
}

func TestClient_ProxyFromEnvironment(t *testing.T) {
	clearProxyEnv(t)
	var viaProxy atomic.Value
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		viaProxy.Store(r.URL.String())
		_, _ = w.Write([]byte("proxied"))
	}))
	defer proxy.Close()
	t.Setenv("HTTP_PROXY", proxy.URL)

	// A non-loopback host: net/http never proxies localhost/127.0.0.1.
	resp, err := NewClient().Get("http://api.hitspec-proxy.test/some/path?x=1", nil)
	require.NoError(t, err)
	assert.Equal(t, "proxied", resp.BodyString())
	assert.Equal(t, "http://api.hitspec-proxy.test/some/path?x=1", viaProxy.Load())
}

func TestClient_NoProxyEnvironmentExcludesHost(t *testing.T) {
	clearProxyEnv(t)
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("request must bypass the proxy, got %s", r.URL)
	}))
	defer proxy.Close()
	t.Setenv("HTTP_PROXY", proxy.URL)
	t.Setenv("NO_PROXY", "api.hitspec-proxy.test")

	_, err := NewClient(WithTimeout(2*time.Second)).Get("http://api.hitspec-proxy.test/", nil)
	require.Error(t, err) // direct connection: the name does not resolve
}

func TestClient_ExplicitProxyBeatsEnvironment(t *testing.T) {
	clearProxyEnv(t)
	var envHits, explicitHits atomic.Int32
	envProxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { envHits.Add(1) }))
	defer envProxy.Close()
	explicit := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		explicitHits.Add(1)
		_, _ = w.Write([]byte("explicit"))
	}))
	defer explicit.Close()
	t.Setenv("HTTP_PROXY", envProxy.URL)

	resp, err := NewClient(WithProxy(explicit.URL)).Get("http://api.hitspec-proxy.test/", nil)
	require.NoError(t, err)
	assert.Equal(t, "explicit", resp.BodyString())
	assert.EqualValues(t, 1, explicitHits.Load())
	assert.EqualValues(t, 0, envHits.Load())
}

func TestClient_InvalidProxyIsReported(t *testing.T) {
	clearProxyEnv(t)
	_, err := NewClientE(WithProxy("http://"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid proxy URL")
}

func newH2Server(t *testing.T, h http.Handler) *httptest.Server {
	t.Helper()
	srv := httptest.NewUnstartedServer(h)
	srv.EnableHTTP2 = true
	srv.StartTLS()
	t.Cleanup(srv.Close)
	return srv
}

func TestClient_NegotiatesHTTP2(t *testing.T) {
	srv := newH2Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, r.Proto)
	}))

	// Including with certificate verification disabled (-k).
	resp, err := NewClient(WithValidateSSL(false)).Get(srv.URL, nil)
	require.NoError(t, err)
	assert.Equal(t, "HTTP/2.0", resp.Proto)
	assert.Equal(t, "HTTP/2.0", resp.BodyString())

	// And with a verified private CA.
	caPath := filepath.Join(t.TempDir(), "ca.pem")
	writePEM(t, caPath, "CERTIFICATE", srv.Certificate().Raw)
	resp, err = NewClient(WithCACert(caPath)).Get(srv.URL, nil)
	require.NoError(t, err)
	assert.Equal(t, "HTTP/2.0", resp.Proto)

	// Opt-out falls back to HTTP/1.1.
	resp, err = NewClient(WithValidateSSL(false), WithHTTP2(false)).Get(srv.URL, nil)
	require.NoError(t, err)
	assert.Equal(t, "HTTP/1.1", resp.Proto)
}

// ---- certificate helpers -------------------------------------------------

type testCA struct {
	cert *x509.Certificate
	key  *ecdsa.PrivateKey
}

func writePEM(t *testing.T, path, typ string, der []byte) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: typ, Bytes: der}), 0o600))
}

func newTestCA(t *testing.T) *testCA {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "hitspec test CA"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)
	cert, err := x509.ParseCertificate(der)
	require.NoError(t, err)
	return &testCA{cert: cert, key: key}
}

// issue returns a leaf certificate signed by the CA, usable for both server
// (127.0.0.1) and client authentication.
func (ca *testCA) issue(t *testing.T, cn string) tls.Certificate {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: cn},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, ca.cert, &key.PublicKey, ca.key)
	require.NoError(t, err)
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
}

// writeKeyPair writes cert and key PEM files and returns their paths.
func writeKeyPair(t *testing.T, dir, name string, c tls.Certificate) (certPath, keyPath string) {
	t.Helper()
	certPath = filepath.Join(dir, name+".pem")
	keyPath = filepath.Join(dir, name+".key")
	writePEM(t, certPath, "CERTIFICATE", c.Certificate[0])
	keyDER, err := x509.MarshalECPrivateKey(c.PrivateKey.(*ecdsa.PrivateKey))
	require.NoError(t, err)
	writePEM(t, keyPath, "EC PRIVATE KEY", keyDER)
	return certPath, keyPath
}

func TestClient_MutualTLS(t *testing.T) {
	ca := newTestCA(t)
	pool := x509.NewCertPool()
	pool.AddCert(ca.cert)

	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, r.TLS.PeerCertificates[0].Subject.CommonName)
	}))
	srv.TLS = &tls.Config{
		Certificates: []tls.Certificate{ca.issue(t, "server")},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    pool,
	}
	srv.StartTLS()
	defer srv.Close()

	dir := t.TempDir()
	caPath := filepath.Join(dir, "ca.pem")
	writePEM(t, caPath, "CERTIFICATE", ca.cert.Raw)
	certPath, keyPath := writeKeyPair(t, dir, "client", ca.issue(t, "alice"))

	client, err := NewClientE(WithCACert(caPath), WithClientCert(certPath, keyPath))
	require.NoError(t, err)
	resp, err := client.Get(srv.URL, nil)
	require.NoError(t, err)
	assert.Equal(t, "alice", resp.BodyString())

	// Trusting the CA but presenting no client certificate is rejected.
	_, err = NewClient(WithCACert(caPath), WithTimeout(5*time.Second)).Get(srv.URL, nil)
	require.Error(t, err)

	// Without the CA the (private) server certificate is not trusted.
	_, err = NewClient(WithClientCert(certPath, keyPath), WithTimeout(5*time.Second)).Get(srv.URL, nil)
	require.Error(t, err)
}

func TestClient_TLSOptionErrors(t *testing.T) {
	dir := t.TempDir()
	ca := newTestCA(t)
	good, goodKey := writeKeyPair(t, dir, "good", ca.issue(t, "x"))
	garbage := filepath.Join(dir, "garbage.pem")
	require.NoError(t, os.WriteFile(garbage, []byte("not pem"), 0o600))
	missing := filepath.Join(dir, "missing.pem")
	// A key that does not belong to the certificate.
	_, otherKey := writeKeyPair(t, dir, "other", ca.issue(t, "y"))

	cases := []struct {
		name string
		opts []ClientOption
		want string
	}{
		{"missing CA", []ClientOption{WithCACert(missing)}, "reading CA certificate"},
		{"garbage CA", []ClientOption{WithCACert(garbage)}, "no valid PEM certificates"},
		{"missing client cert", []ClientOption{WithClientCert(missing, goodKey)}, "loading client certificate"},
		{"garbage client cert", []ClientOption{WithClientCert(garbage, goodKey)}, "loading client certificate"},
		{"mismatched key", []ClientOption{WithClientCert(good, otherKey)}, "loading client certificate"},
		{"cert without key", []ClientOption{WithClientCert(good, "")}, "configured together"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewClientE(tc.opts...)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)

			// NewClient stays infallible; the error surfaces on use.
			_, err = NewClient(tc.opts...).Get("http://127.0.0.1:1/", nil)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}

func TestClient_HostHeaderOverridesRequestHost(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, r.Host)
	}))
	defer server.Close()

	for _, key := range []string{"Host", "host"} {
		resp, err := NewClient().Get(server.URL, map[string]string{key: "custom.example.com"})
		require.NoError(t, err)
		assert.Equal(t, "custom.example.com", resp.BodyString(), "header key %q", key)
	}

	client := NewClient(WithDefaultHeader("Host", "default.example.com"))
	resp, err := client.Get(server.URL, nil)
	require.NoError(t, err)
	assert.Equal(t, "default.example.com", resp.BodyString())
}

func gzipBytes(t *testing.T, s string) []byte {
	t.Helper()
	var b strings.Builder
	zw := gzip.NewWriter(&b)
	_, err := zw.Write([]byte(s))
	require.NoError(t, err)
	require.NoError(t, zw.Close())
	return []byte(b.String())
}

func TestClient_ExplicitAcceptEncodingIsDecoded(t *testing.T) {
	payload := strings.Repeat(`{"hello":"world"}`, 200)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.Header.Get("Accept-Encoding"), "gzip"):
			w.Header().Set("Content-Encoding", "gzip")
			_, _ = w.Write(gzipBytes(t, payload))
		case r.Header.Get("Accept-Encoding") == "deflate":
			w.Header().Set("Content-Encoding", "deflate")
			zw := zlib.NewWriter(w)
			_, _ = zw.Write([]byte(payload))
			_ = zw.Close()
		default:
			_, _ = w.Write([]byte(payload))
		}
	}))
	defer server.Close()

	for _, enc := range []string{"gzip", "deflate", "identity", ""} {
		t.Run("enc="+enc, func(t *testing.T) {
			headers := map[string]string{}
			if enc != "" {
				headers["Accept-Encoding"] = enc
			}
			resp, err := NewClient().Get(server.URL, headers)
			require.NoError(t, err)
			assert.Equal(t, payload, resp.BodyString())
			if enc == "gzip" || enc == "deflate" {
				assert.Less(t, resp.RawBodySize, int64(len(payload)), "raw size is the compressed size")
				assert.Equal(t, enc, resp.Header("Content-Encoding"))
			} else {
				assert.Equal(t, int64(len(payload)), resp.RawBodySize)
			}
		})
	}
}

func TestClient_CorruptGzipFails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Encoding", "gzip")
		_, _ = w.Write([]byte("definitely not gzip"))
	}))
	defer server.Close()
	_, err := NewClient().Get(server.URL, map[string]string{"Accept-Encoding": "gzip"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "decoding gzip response body")
}

func TestClient_BodyLimit(t *testing.T) {
	assert.Equal(t, DefaultMaxBodyBytes, NewClient().maxBodyBytes, "default limit")
	assert.EqualValues(t, 7, NewClient(WithMaxBodyBytes(7)).maxBodyBytes)
	assert.EqualValues(t, 0, NewClient(WithMaxBodyBytes(-1)).maxBodyBytes, "negative means unlimited")

	t.Run("declared length is rejected before reading", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Length", "1000000")
			w.WriteHeader(200)
			_, _ = w.Write([]byte("tiny")) // never completes the declared length
		}))
		defer server.Close()
		_, err := NewClient(WithMaxBodyBytes(100)).Get(server.URL, nil)
		var tooLarge *ResponseTooLargeError
		require.ErrorAs(t, err, &tooLarge)
		assert.Contains(t, err.Error(), "response body exceeds 100")
	})

	t.Run("endless chunked stream is cut off", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			chunk := []byte(strings.Repeat("x", 4096))
			for r.Context().Err() == nil {
				if _, err := w.Write(chunk); err != nil {
					return
				}
			}
		}))
		defer server.Close()
		start := time.Now()
		_, err := NewClient(WithMaxBodyBytes(64<<10)).Get(server.URL, nil)
		var tooLarge *ResponseTooLargeError
		require.ErrorAs(t, err, &tooLarge)
		assert.Less(t, time.Since(start), 5*time.Second)
	})

	t.Run("HEAD with a large Content-Length is fine", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Length", "1000000")
		}))
		defer server.Close()
		resp, err := NewClient(WithMaxBodyBytes(100)).Do(&Request{Method: "HEAD", URL: server.URL})
		require.NoError(t, err)
		assert.Equal(t, 200, resp.StatusCode)
	})

	t.Run("decoded gzip bomb is bounded", func(t *testing.T) {
		bomb := gzipBytes(t, strings.Repeat("\x00", 4<<20))
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Encoding", "gzip")
			_, _ = w.Write(bomb)
		}))
		defer server.Close()
		require.Less(t, len(bomb), 10_000)
		_, err := NewClient(WithMaxBodyBytes(1<<20)).Get(server.URL, map[string]string{"Accept-Encoding": "gzip"})
		var tooLarge *ResponseTooLargeError
		require.ErrorAs(t, err, &tooLarge)
	})
}

func TestClient_CookieJar(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/login":
			http.SetCookie(w, &http.Cookie{Name: "session", Value: "abc123", Path: "/"})
		case "/me":
			c, err := r.Cookie("session")
			if err != nil {
				http.Error(w, "no session", http.StatusUnauthorized)
				return
			}
			_, _ = w.Write([]byte("hello " + c.Value))
		}
	}))
	defer server.Close()

	// Default: cookies are not carried over (unchanged behavior).
	plain := NewClient()
	_, err := plain.Get(server.URL+"/login", nil)
	require.NoError(t, err)
	resp, err := plain.Get(server.URL+"/me", nil)
	require.NoError(t, err)
	assert.Equal(t, 401, resp.StatusCode)
	assert.Nil(t, plain.CookieJar())

	for name, client := range map[string]*Client{"WithCookies": NewClient(WithCookies(true))} {
		_, err = client.Get(server.URL+"/login", nil)
		require.NoError(t, err, name)
		resp, err = client.Get(server.URL+"/me", nil)
		require.NoError(t, err, name)
		assert.Equal(t, 200, resp.StatusCode, name)
		assert.Equal(t, "hello abc123", resp.BodyString(), name)
	}

	jar, err := cookiejar.New(nil)
	require.NoError(t, err)
	shared := NewClient(WithCookieJar(jar))
	_, err = shared.Get(server.URL+"/login", nil)
	require.NoError(t, err)
	u := server.URL + "/me"
	parsed, _ := http.NewRequest("GET", u, nil)
	assert.Len(t, jar.Cookies(parsed.URL), 1)
}

func TestClient_RedirectHeaders(t *testing.T) {
	var seen atomic.Value // http.Header of the final hop
	final := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen.Store(r.Header.Clone())
		_, _ = w.Write([]byte("final"))
	}))
	defer final.Close()
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/cross":
			http.Redirect(w, r, final.URL+"/landed", http.StatusFound)
		case "/same":
			http.Redirect(w, r, "/landed", http.StatusFound)
		default:
			seen.Store(r.Header.Clone())
			_, _ = w.Write([]byte("same-origin"))
		}
	}))
	defer origin.Close()

	headers := map[string]string{
		"X-Api-Key":       "secret-key",
		"Authorization":   "Bearer tok",
		"Cookie":          "sid=1",
		"Accept":          "application/json",
		"X-Request-Trace": "t-1",
	}

	t.Run("cross-origin drops user headers", func(t *testing.T) {
		resp, err := NewClient().Get(origin.URL+"/cross", headers)
		require.NoError(t, err)
		assert.Equal(t, "final", resp.BodyString())
		h := seen.Load().(http.Header)
		assert.Empty(t, h.Get("X-Api-Key"))
		assert.Empty(t, h.Get("Authorization"))
		assert.Empty(t, h.Get("Cookie"))
		assert.Empty(t, h.Get("X-Request-Trace"))
		assert.Equal(t, "application/json", h.Get("Accept"), "content negotiation survives")
	})

	t.Run("same origin keeps everything", func(t *testing.T) {
		resp, err := NewClient().Get(origin.URL+"/same", headers)
		require.NoError(t, err)
		assert.Equal(t, "same-origin", resp.BodyString())
		h := seen.Load().(http.Header)
		assert.Equal(t, "secret-key", h.Get("X-Api-Key"))
		assert.Equal(t, "Bearer tok", h.Get("Authorization"))
		assert.Equal(t, "sid=1", h.Get("Cookie"))
	})

	t.Run("opt in to forwarding", func(t *testing.T) {
		_, err := NewClient(WithForwardRedirectHeaders(true)).Get(origin.URL+"/cross", map[string]string{"X-Api-Key": "k"})
		require.NoError(t, err)
		assert.Equal(t, "k", seen.Load().(http.Header).Get("X-Api-Key"))
	})
}

func TestSameOrigin(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"http://a.com/x", "http://a.com/y", true},
		{"http://a.com", "http://a.com:80/y", true},
		{"https://a.com", "https://a.com:443", true},
		{"http://a.com", "http://a.com:8080", false},
		{"http://a.com", "https://a.com", false},
		{"http://a.com", "http://api.a.com", false},
		{"http://A.com", "http://a.COM", true},
	}
	for _, tc := range cases {
		pa, _ := http.NewRequest("GET", tc.a, nil)
		pb, _ := http.NewRequest("GET", tc.b, nil)
		assert.Equal(t, tc.want, sameOrigin(pa.URL, pb.URL), "%s vs %s", tc.a, tc.b)
	}
}
