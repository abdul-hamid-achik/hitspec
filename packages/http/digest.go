package http

import (
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
)

// DigestAuth contains the parameters needed for digest authentication
type DigestAuth struct {
	Username string
	Password string
	Realm    string
	Nonce    string
	URI      string
	Qop      string
	Nc       string
	Cnonce   string
	Opaque   string
	Method   string
}

// ParseWWWAuthenticate parses the WWW-Authenticate header from a 401 response
func ParseWWWAuthenticate(header string) map[string]string {
	result := make(map[string]string)

	// Remove "Digest " prefix
	header = strings.TrimPrefix(header, "Digest ")

	// Parse key="value" pairs
	parts := strings.Split(header, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if idx := strings.Index(part, "="); idx != -1 {
			key := strings.TrimSpace(part[:idx])
			value := strings.TrimSpace(part[idx+1:])
			// Remove quotes if present
			value = strings.Trim(value, `"`)
			result[key] = value
		}
	}

	return result
}

// ComputeDigestResponse calculates the digest response hash
func (d *DigestAuth) ComputeDigestResponse() string {
	// HA1 = MD5(username:realm:password)
	ha1 := md5Hash(fmt.Sprintf("%s:%s:%s", d.Username, d.Realm, d.Password))

	// HA2 = MD5(method:uri)
	ha2 := md5Hash(fmt.Sprintf("%s:%s", d.Method, d.URI))

	// Response calculation depends on qop
	var response string
	if d.Qop == "auth" || d.Qop == "auth-int" {
		// response = MD5(HA1:nonce:nc:cnonce:qop:HA2)
		response = md5Hash(fmt.Sprintf("%s:%s:%s:%s:%s:%s", ha1, d.Nonce, d.Nc, d.Cnonce, d.Qop, ha2))
	} else {
		// response = MD5(HA1:nonce:HA2)
		response = md5Hash(fmt.Sprintf("%s:%s:%s", ha1, d.Nonce, ha2))
	}

	return response
}

// BuildAuthorizationHeader creates the Authorization header value
func (d *DigestAuth) BuildAuthorizationHeader() string {
	response := d.ComputeDigestResponse()

	parts := []string{
		fmt.Sprintf(`username="%s"`, d.Username),
		fmt.Sprintf(`realm="%s"`, d.Realm),
		fmt.Sprintf(`nonce="%s"`, d.Nonce),
		fmt.Sprintf(`uri="%s"`, d.URI),
		fmt.Sprintf(`response="%s"`, response),
	}

	if d.Qop != "" {
		parts = append(parts, fmt.Sprintf(`qop=%s`, d.Qop))
		parts = append(parts, fmt.Sprintf(`nc=%s`, d.Nc))
		parts = append(parts, fmt.Sprintf(`cnonce="%s"`, d.Cnonce))
	}

	if d.Opaque != "" {
		parts = append(parts, fmt.Sprintf(`opaque="%s"`, d.Opaque))
	}

	return "Digest " + strings.Join(parts, ", ")
}

// GenerateCnonce generates a random client nonce
func GenerateCnonce() (string, error) {
	b := make([]byte, 8)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func md5Hash(s string) string {
	h := md5.New()
	h.Write([]byte(s))
	return hex.EncodeToString(h.Sum(nil))
}
