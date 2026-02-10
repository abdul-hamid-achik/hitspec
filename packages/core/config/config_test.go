package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	c := DefaultConfig()

	if c.DefaultEnvironment != "dev" {
		t.Errorf("DefaultEnvironment = %q, want %q", c.DefaultEnvironment, "dev")
	}
	if !c.GetFollowRedirects() {
		t.Error("FollowRedirects should default to true")
	}
	if !c.GetValidateSSL() {
		t.Error("ValidateSSL should default to true")
	}
	if c.GetParallel() {
		t.Error("Parallel should default to false")
	}
	if c.GetBail() {
		t.Error("Bail should default to false")
	}
	if c.GetVerbose() {
		t.Error("Verbose should default to false")
	}
	if c.GetNoColor() {
		t.Error("NoColor should default to false")
	}
	if c.Timeout == 0 {
		t.Error("Timeout should have a default value")
	}
}

func TestGetBool(t *testing.T) {
	trueVal := true
	falseVal := false

	tests := []struct {
		name       string
		b          *bool
		defaultVal bool
		want       bool
	}{
		{"nil true default", nil, true, true},
		{"nil false default", nil, false, false},
		{"true pointer", &trueVal, false, true},
		{"false pointer", &falseVal, true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getBool(tt.b, tt.defaultVal)
			if got != tt.want {
				t.Errorf("getBool = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMerge_Nil(t *testing.T) {
	c := DefaultConfig()
	result := c.Merge(nil)
	if result.DefaultEnvironment != "dev" {
		t.Error("merge with nil should return original")
	}
}

func TestMerge_Overrides(t *testing.T) {
	base := DefaultConfig()
	overlay := &Config{
		DefaultEnvironment: "staging",
		Timeout:            5000,
		Retries:            3,
		RetryDelay:         1000,
		MaxRedirects:       5,
		Proxy:              "http://proxy:8080",
		OutputDir:          "/tmp/output",
		Concurrency:        4,
		FollowRedirects:    BoolPtr(false),
		ValidateSSL:        BoolPtr(false),
		Parallel:           BoolPtr(true),
		Bail:               BoolPtr(true),
		Verbose:            BoolPtr(true),
		NoColor:            BoolPtr(true),
	}

	result := base.Merge(overlay)

	if result.DefaultEnvironment != "staging" {
		t.Errorf("DefaultEnvironment = %q, want %q", result.DefaultEnvironment, "staging")
	}
	if result.Timeout != 5000 {
		t.Errorf("Timeout = %d, want 5000", result.Timeout)
	}
	if result.Retries != 3 {
		t.Errorf("Retries = %d, want 3", result.Retries)
	}
	if result.RetryDelay != 1000 {
		t.Errorf("RetryDelay = %d, want 1000", result.RetryDelay)
	}
	if result.MaxRedirects != 5 {
		t.Errorf("MaxRedirects = %d, want 5", result.MaxRedirects)
	}
	if result.Proxy != "http://proxy:8080" {
		t.Errorf("Proxy = %q, want %q", result.Proxy, "http://proxy:8080")
	}
	if result.Concurrency != 4 {
		t.Errorf("Concurrency = %d, want 4", result.Concurrency)
	}
	if result.GetFollowRedirects() {
		t.Error("FollowRedirects should be false after merge")
	}
	if result.GetValidateSSL() {
		t.Error("ValidateSSL should be false after merge")
	}
	if !result.GetParallel() {
		t.Error("Parallel should be true after merge")
	}
	if !result.GetBail() {
		t.Error("Bail should be true after merge")
	}
}

func TestMerge_Headers(t *testing.T) {
	base := DefaultConfig()
	base.Headers = map[string]string{"Accept": "application/json"}

	overlay := &Config{
		Headers: map[string]string{"X-Custom": "value"},
	}

	result := base.Merge(overlay)
	if result.Headers["Accept"] != "application/json" {
		t.Error("base header should be preserved")
	}
	if result.Headers["X-Custom"] != "value" {
		t.Error("overlay header should be added")
	}
}

func TestMerge_Environments(t *testing.T) {
	base := DefaultConfig()
	base.Environments = map[string]map[string]any{
		"dev": {"baseUrl": "http://localhost"},
	}

	overlay := &Config{
		Environments: map[string]map[string]any{
			"dev":     {"apiKey": "test-key"},
			"staging": {"baseUrl": "http://staging"},
		},
	}

	result := base.Merge(overlay)
	if result.Environments["dev"]["baseUrl"] != "http://localhost" {
		t.Error("existing env var should be preserved")
	}
	if result.Environments["dev"]["apiKey"] != "test-key" {
		t.Error("new env var should be added")
	}
	if result.Environments["staging"]["baseUrl"] != "http://staging" {
		t.Error("new environment should be added")
	}
}

func TestMerge_Reporters(t *testing.T) {
	base := DefaultConfig()
	overlay := &Config{
		Reporters: []string{"json", "junit"},
	}
	result := base.Merge(overlay)
	if len(result.Reporters) != 2 || result.Reporters[0] != "json" {
		t.Errorf("reporters = %v, want [json junit]", result.Reporters)
	}
}

func TestLoadConfig_FromFile(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "hitspec.yaml")
	content := `defaultEnvironment: production
timeout: 10000
retries: 2
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.DefaultEnvironment != "production" {
		t.Errorf("DefaultEnvironment = %q, want %q", cfg.DefaultEnvironment, "production")
	}
	if cfg.Timeout != 10000 {
		t.Errorf("Timeout = %d, want 10000", cfg.Timeout)
	}
	if cfg.Retries != 2 {
		t.Errorf("Retries = %d, want 2", cfg.Retries)
	}
}

func TestLoadConfig_NoFile(t *testing.T) {
	cfg, err := LoadConfig("")
	if err != nil {
		t.Fatal(err)
	}
	// Should return defaults when no file found in cwd
	if cfg.DefaultEnvironment != "dev" {
		t.Errorf("DefaultEnvironment = %q, want %q", cfg.DefaultEnvironment, "dev")
	}
}

func TestFindAndLoadConfig_SearchesFilenames(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "hitspec.yml")
	content := `timeout: 7777
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := FindAndLoadConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Timeout != 7777 {
		t.Errorf("Timeout = %d, want 7777", cfg.Timeout)
	}
}

func TestSaveConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hitspec.yaml")

	cfg := DefaultConfig()
	cfg.DefaultEnvironment = "test-env"
	if err := cfg.SaveConfig(path); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.DefaultEnvironment != "test-env" {
		t.Errorf("round-trip DefaultEnvironment = %q, want %q", loaded.DefaultEnvironment, "test-env")
	}
}

func TestLoadConfig_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "hitspec.yaml")
	if err := os.WriteFile(configPath, []byte("{{invalid yaml"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadConfig(configPath)
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestLoadConfig_MissingFile(t *testing.T) {
	_, err := LoadConfig("/nonexistent/path/hitspec.yaml")
	if err == nil {
		t.Error("expected error for missing file")
	}
}
