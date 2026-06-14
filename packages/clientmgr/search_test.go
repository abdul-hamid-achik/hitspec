package clientmgr

import (
	"context"
	"testing"
)

func TestSearchRequests(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	if _, err := m.CreateFile(ctx, "users.http", "### List users\n# @tags smoke\nGET https://api.example.com/users\n\n### Create user\nPOST https://api.example.com/users\n"); err != nil {
		t.Fatalf("create users: %v", err)
	}
	if _, err := m.CreateFile(ctx, "auth.http", "### Login\nPOST https://api.example.com/login\n"); err != nil {
		t.Fatalf("create auth: %v", err)
	}

	// Match by request name.
	if res, _ := m.SearchRequests(ctx, "login"); len(res) != 1 || res[0].RequestName != "Login" {
		t.Fatalf("search 'login' -> %+v", res)
	}
	// Match by URL substring (both /users requests).
	if res, _ := m.SearchRequests(ctx, "users"); len(res) != 2 {
		t.Fatalf("search 'users' -> want 2, got %d", len(res))
	}
	// Match by tag.
	if res, _ := m.SearchRequests(ctx, "smoke"); len(res) != 1 {
		t.Fatalf("search 'smoke' -> want 1, got %d", len(res))
	}
	// Case-insensitive.
	if res, _ := m.SearchRequests(ctx, "LOGIN"); len(res) != 1 {
		t.Fatalf("search 'LOGIN' should be case-insensitive, got %d", len(res))
	}
	// Empty query returns nothing.
	if res, _ := m.SearchRequests(ctx, "   "); res != nil {
		t.Fatalf("empty query should return nil, got %+v", res)
	}
}
