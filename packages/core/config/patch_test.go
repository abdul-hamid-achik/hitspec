package config

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

func TestPatchDecodesKeyPresence(t *testing.T) {
	var p Patch
	if err := json.Unmarshal([]byte(`{"retries":0,"proxy":""}`), &p); err != nil {
		t.Fatal(err)
	}
	if p.Retries == nil || *p.Retries != 0 || p.Proxy == nil || *p.Proxy != "" {
		t.Fatalf("explicit zero/empty must be present: %+v", p)
	}
	if p.Timeout != nil || p.RetryDelay != nil || p.MaxRedirects != nil || p.Headers != nil {
		t.Fatalf("absent keys must stay nil: %+v", p)
	}
}

func TestApplyExplicitZeroAndClear(t *testing.T) {
	c := DefaultConfig()
	c.Retries = 3
	c.Proxy = "http://proxy:8080"
	zero, empty := 0, ""
	if err := c.Apply(Patch{Retries: &zero, Proxy: &empty}); err != nil {
		t.Fatal(err)
	}
	if c.Retries != 0 || c.Proxy != "" {
		t.Fatalf("retries=%d proxy=%q, want 0 and empty", c.Retries, c.Proxy)
	}
}

func TestApplyRetryDelayMaxRedirectsAndDefaults(t *testing.T) {
	c := DefaultConfig()
	delay, redirects := 250, 3
	if err := c.Apply(Patch{RetryDelay: &delay, MaxRedirects: &redirects}); err != nil {
		t.Fatal(err)
	}
	if c.RetryDelay != 250 || c.MaxRedirects != 3 {
		t.Fatalf("%+v", c)
	}
	zero := 0
	if err := c.Apply(Patch{RetryDelay: &zero, MaxRedirects: &zero, Timeout: &zero}); err != nil {
		t.Fatal(err)
	}
	if c.RetryDelay != DefaultRetryDelayMs || c.MaxRedirects != DefaultMaxRedirects || c.Timeout != DefaultTimeoutMs {
		t.Fatalf("explicit 0 must reset to the default: %+v", c)
	}
}

func TestApplyRejectsNegativeAndLeavesConfigUntouched(t *testing.T) {
	c := DefaultConfig()
	env, neg := "prod", -1
	if err := c.Apply(Patch{DefaultEnvironment: &env, Timeout: &neg}); err == nil {
		t.Fatal("negative timeout accepted")
	}
	if c.DefaultEnvironment != "dev" {
		t.Fatalf("a rejected patch changed the config: %+v", c)
	}
}

// What Apply produces survives a save/load cycle (omitempty drops zero values,
// which is why zero-default settings reset instead of persisting 0).
func TestApplyRoundTripsThroughYAML(t *testing.T) {
	c := DefaultConfig()
	c.Retries = 5
	c.Proxy = "http://p:1"
	zero, empty := 0, ""
	if err := c.Apply(Patch{Retries: &zero, Proxy: &empty}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "hitspec.yaml")
	if err := c.SaveConfig(path); err != nil {
		t.Fatal(err)
	}
	got, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Retries != 0 || got.Proxy != "" {
		t.Fatalf("reloaded retries=%d proxy=%q", got.Retries, got.Proxy)
	}
}
