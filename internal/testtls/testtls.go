// Package testtls generates throwaway TLS material for tests: a CA, a server
// certificate for 127.0.0.1/localhost and a client certificate, all written as
// PEM files into a test's temporary directory. Nothing here is committed and
// every key is created in the test process.
package testtls

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Material is a generated CA with a server and a client certificate it signed.
type Material struct {
	CAPath         string // PEM CA certificate
	ServerCertPath string
	ServerKeyPath  string
	ClientCertPath string
	ClientKeyPath  string

	ca     *x509.Certificate
	caKey  *ecdsa.PrivateKey
	caPool *x509.CertPool
	server tls.Certificate
}

// New generates the material into t.TempDir().
func New(t testing.TB) *Material {
	t.Helper()
	dir := t.TempDir()

	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	must(t, err)
	caTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "hitspec test CA"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTmpl, caTmpl, &caKey.PublicKey, caKey)
	must(t, err)
	ca, err := x509.ParseCertificate(caDER)
	must(t, err)

	m := &Material{ca: ca, caKey: caKey, caPool: x509.NewCertPool()}
	m.caPool.AddCert(ca)
	m.CAPath = writePEM(t, dir, "ca.pem", "CERTIFICATE", caDER)

	m.ServerCertPath, m.ServerKeyPath = m.issue(t, dir, "server", 2, x509.ExtKeyUsageServerAuth)
	m.ClientCertPath, m.ClientKeyPath = m.issue(t, dir, "client", 3, x509.ExtKeyUsageClientAuth)

	m.server, err = tls.LoadX509KeyPair(m.ServerCertPath, m.ServerKeyPath)
	must(t, err)
	return m
}

func (m *Material) issue(t testing.TB, dir, name string, serial int64, usage x509.ExtKeyUsage) (certPath, keyPath string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	must(t, err)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(serial),
		Subject:      pkix.Name{CommonName: name},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{usage},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")},
		DNSNames:     []string{"localhost"},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, m.ca, &key.PublicKey, m.caKey)
	must(t, err)
	keyDER, err := x509.MarshalECPrivateKey(key)
	must(t, err)
	return writePEM(t, dir, name+".pem", "CERTIFICATE", der), writePEM(t, dir, name+"-key.pem", "EC PRIVATE KEY", keyDER)
}

// Server starts an HTTPS test server presenting the server certificate. With
// requireClientCert it only accepts clients whose certificate the CA signed.
// h may be nil (200 OK). HTTP/2 is enabled; the server closes with the test.
func (m *Material) Server(t testing.TB, h http.Handler, requireClientCert bool) *httptest.Server {
	t.Helper()
	if h == nil {
		h = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	}
	srv := httptest.NewUnstartedServer(h)
	srv.TLS = &tls.Config{Certificates: []tls.Certificate{m.server}}
	if requireClientCert {
		srv.TLS.ClientAuth = tls.RequireAndVerifyClientCert
		srv.TLS.ClientCAs = m.caPool
	}
	srv.EnableHTTP2 = true
	srv.StartTLS()
	t.Cleanup(srv.Close)
	return srv
}

func writePEM(t testing.TB, dir, name, typ string, der []byte) string {
	t.Helper()
	path := filepath.Join(dir, name)
	must(t, os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: typ, Bytes: der}), 0o600))
	return path
}

func must(t testing.TB, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
