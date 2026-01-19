package env

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDotEnv(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected map[string]string
	}{
		{
			name:    "simple key-value",
			content: "API_KEY=secret123",
			expected: map[string]string{
				"API_KEY": "secret123",
			},
		},
		{
			name:    "multiple keys",
			content: "KEY1=value1\nKEY2=value2\nKEY3=value3",
			expected: map[string]string{
				"KEY1": "value1",
				"KEY2": "value2",
				"KEY3": "value3",
			},
		},
		{
			name:    "double quoted value",
			content: `API_KEY="secret with spaces"`,
			expected: map[string]string{
				"API_KEY": "secret with spaces",
			},
		},
		{
			name:    "single quoted value",
			content: `API_KEY='secret with spaces'`,
			expected: map[string]string{
				"API_KEY": "secret with spaces",
			},
		},
		{
			name:    "comments are skipped",
			content: "# This is a comment\nAPI_KEY=secret",
			expected: map[string]string{
				"API_KEY": "secret",
			},
		},
		{
			name:    "empty lines are skipped",
			content: "KEY1=value1\n\n\nKEY2=value2",
			expected: map[string]string{
				"KEY1": "value1",
				"KEY2": "value2",
			},
		},
		{
			name:    "whitespace trimmed",
			content: "  API_KEY  =  secret  ",
			expected: map[string]string{
				"API_KEY": "secret",
			},
		},
		{
			name:    "value with equals sign",
			content: "CONNECTION=postgres://user:pass@host/db?ssl=true",
			expected: map[string]string{
				"CONNECTION": "postgres://user:pass@host/db?ssl=true",
			},
		},
		{
			name:     "empty file",
			content:  "",
			expected: map[string]string{},
		},
		{
			name:     "only comments",
			content:  "# comment 1\n# comment 2",
			expected: map[string]string{},
		},
		{
			name:    "inline comment not supported",
			content: "API_KEY=secret # this is included",
			expected: map[string]string{
				"API_KEY": "secret # this is included",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp file
			tmpDir := t.TempDir()
			envFile := filepath.Join(tmpDir, ".env")
			if err := os.WriteFile(envFile, []byte(tt.content), 0644); err != nil {
				t.Fatalf("failed to write temp file: %v", err)
			}

			result, err := LoadDotEnv(envFile)
			if err != nil {
				t.Fatalf("LoadDotEnv() error = %v", err)
			}

			if len(result) != len(tt.expected) {
				t.Errorf("LoadDotEnv() returned %d keys, want %d", len(result), len(tt.expected))
			}

			for k, v := range tt.expected {
				if got, ok := result[k]; !ok {
					t.Errorf("LoadDotEnv() missing key %q", k)
				} else if got != v {
					t.Errorf("LoadDotEnv()[%q] = %q, want %q", k, got, v)
				}
			}
		})
	}
}

func TestLoadDotEnvFileNotFound(t *testing.T) {
	_, err := LoadDotEnv("/nonexistent/path/.env")
	if err == nil {
		t.Error("LoadDotEnv() expected error for non-existent file")
	}
}
