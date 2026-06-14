package clientmgr

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/abdul-hamid-achik/hitspec/packages/core/runner"
)

var cookieMu sync.Mutex

// ListCookies returns the local TUI cookie store.
func (m *Manager) ListCookies(ctx context.Context) ([]CookieDTO, error) {
	_ = ctx
	return m.loadCookies()
}

// PutCookie creates or replaces a local cookie record.
func (m *Manager) PutCookie(ctx context.Context, cookie CookieDTO) ([]CookieDTO, error) {
	_ = ctx
	if err := m.requireWritable(); err != nil {
		return nil, err
	}
	cookies, err := m.loadCookies()
	if err != nil {
		return nil, err
	}
	replaced := false
	for i, existing := range cookies {
		if sameCookie(existing, cookie) {
			cookies[i] = cookie
			replaced = true
			break
		}
	}
	if !replaced {
		cookies = append(cookies, cookie)
	}
	return cookies, m.saveCookies(cookies)
}

// DeleteCookie removes a local cookie record by domain/path/name.
func (m *Manager) DeleteCookie(ctx context.Context, domain, path, name string) ([]CookieDTO, error) {
	_ = ctx
	if err := m.requireWritable(); err != nil {
		return nil, err
	}
	cookies, err := m.loadCookies()
	if err != nil {
		return nil, err
	}
	next := cookies[:0]
	target := CookieDTO{Domain: domain, Path: path, Name: name}
	for _, c := range cookies {
		if !sameCookie(c, target) {
			next = append(next, c)
		}
	}
	return next, m.saveCookies(next)
}

// captureCookies persists Set-Cookie headers from a run into the local cookie
// store. It reads the RAW runner result (not the DTO), because the DTO's response
// headers are redacted — reading the DTO would only ever see "[REDACTED]".
func (m *Manager) captureCookies(result *runner.RunResult) {
	if result == nil {
		return
	}
	cookies, err := m.loadCookies()
	if err != nil {
		return
	}
	changed := false
	for _, rr := range result.Results {
		if rr.Response == nil || rr.Request == nil {
			continue
		}
		setCookie := rr.Response.Headers["Set-Cookie"]
		if setCookie == "" {
			setCookie = rr.Response.Headers["set-cookie"]
		}
		if setCookie == "" {
			continue
		}
		u, _ := url.Parse(rr.Request.URL)
		domain := u.Hostname()
		for _, raw := range splitSetCookie(setCookie) {
			h := http.Header{}
			h.Add("Set-Cookie", raw)
			resp := http.Response{Header: h}
			for _, c := range resp.Cookies() {
				dto := CookieDTO{
					Domain:   domain,
					Path:     c.Path,
					Name:     c.Name,
					Value:    c.Value,
					Secure:   c.Secure,
					HTTPOnly: c.HttpOnly,
				}
				if dto.Path == "" {
					dto.Path = "/"
				}
				if !c.Expires.IsZero() {
					dto.ExpiresAt = c.Expires.UTC().Format(time.RFC3339)
				}
				replaced := false
				for i, existing := range cookies {
					if sameCookie(existing, dto) {
						cookies[i] = dto
						replaced = true
						break
					}
				}
				if !replaced {
					cookies = append(cookies, dto)
				}
				changed = true
			}
		}
	}
	if changed {
		_ = m.saveCookies(cookies)
	}
}

func (m *Manager) loadCookies() ([]CookieDTO, error) {
	cookieMu.Lock()
	defer cookieMu.Unlock()
	path, err := cookieStorePath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return []CookieDTO{}, nil
	}
	if err != nil {
		return nil, err
	}
	var cookies []CookieDTO
	if len(data) == 0 {
		return []CookieDTO{}, nil
	}
	if err := json.Unmarshal(data, &cookies); err != nil {
		return nil, err
	}
	return cookies, nil
}

func (m *Manager) saveCookies(cookies []CookieDTO) error {
	cookieMu.Lock()
	defer cookieMu.Unlock()
	path, err := cookieStorePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cookies, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o600)
}

func cookieStorePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".hitspec", "tui-cookies.json"), nil
}

func sameCookie(a, b CookieDTO) bool {
	return strings.EqualFold(a.Domain, b.Domain) && a.Path == b.Path && a.Name == b.Name
}

func splitSetCookie(header string) []string {
	if !strings.Contains(header, ",") {
		return []string{strings.TrimSpace(header)}
	}
	parts := strings.Split(header, ",")
	out := make([]string, 0, len(parts))
	var current string
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		lower := strings.ToLower(trimmed)
		if current != "" && (strings.HasPrefix(lower, "expires=") || !strings.Contains(trimmed, "=")) {
			current += ", " + trimmed
			continue
		}
		if current != "" {
			out = append(out, current)
		}
		current = trimmed
	}
	if current != "" {
		out = append(out, current)
	}
	return out
}
