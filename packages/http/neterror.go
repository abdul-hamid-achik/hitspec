package http

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"syscall"
)

// ConnectTimeoutError reports that the request deadline expired before a
// connection was established (a host that drops packets, a stalled TLS
// handshake). Its message is the underlying error's. IsNetworkError accepts it
// even though the cause is a context deadline: nothing ever connected.
type ConnectTimeoutError struct{ err error }

func (e *ConnectTimeoutError) Error() string { return e.err.Error() }
func (e *ConnectTimeoutError) Unwrap() error { return e.err }

// IsNetworkError reports whether err means a request never got a response
// because the connection itself failed: connect errors (refused, unreachable),
// DNS resolution failures, TLS handshake or certificate errors, a connection
// reset or closed before any response arrived, and proxy connect errors.
// `hitspec run` maps it to exit code 4.
//
// It is deliberately narrow. A request that timed out after the connection was
// established, a cancelled request, an error reading the body of a response
// that did arrive, and every other failure (assertions, bad URL, hooks, ...)
// are not network errors. Errors from the OAuth2 token request qualify when the
// token endpoint cannot be reached: the client wraps them with %w.
func IsNetworkError(err error) bool {
	if err == nil {
		return false
	}
	// Ctrl-C or a cancelled run is not a network problem, even when the
	// cancellation interrupts a dial.
	if errors.Is(err, context.Canceled) {
		return false
	}
	// A response body that broke part-way means a response did arrive.
	var bodyErr *BodyReadError
	if errors.As(err, &bodyErr) {
		return false
	}

	var connectTimeout *ConnectTimeoutError
	if errors.As(err, &connectTimeout) {
		return true
	}

	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return true
	}
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		switch opErr.Op {
		case "dial", "proxyconnect":
			return true
		}
	}
	if isTLSError(err) {
		return true
	}

	// net/http reports a reset or closed connection as an io.EOF/ECONNRESET
	// inside a *url.Error. Requiring the *url.Error keeps unrelated EOFs (a
	// truncated file, a hook's pipe) from counting.
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return errors.Is(urlErr, io.EOF) ||
			errors.Is(urlErr, io.ErrUnexpectedEOF) ||
			errors.Is(urlErr, syscall.ECONNRESET) ||
			errors.Is(urlErr, syscall.ECONNREFUSED) ||
			errors.Is(urlErr, syscall.ECONNABORTED) ||
			errors.Is(urlErr, syscall.EPIPE)
	}
	return false
}

// isTLSError reports handshake and certificate failures.
func isTLSError(err error) bool {
	var (
		recordErr  tls.RecordHeaderError
		alertErr   tls.AlertError
		certVerify *tls.CertificateVerificationError
		unknownCA  x509.UnknownAuthorityError
		invalid    x509.CertificateInvalidError
		hostname   x509.HostnameError
	)
	switch {
	case errors.As(err, &recordErr),
		errors.As(err, &alertErr),
		errors.As(err, &certVerify),
		errors.As(err, &unknownCA),
		errors.As(err, &invalid),
		errors.As(err, &hostname):
		return true
	}
	// https:// sent to a plain-HTTP server: net/http replaces the
	// RecordHeaderError with this sentinel.
	return errors.Is(err, http.ErrSchemeMismatch)
}
