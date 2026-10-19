package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// repoRoot is the module root (this package is packages/core/config).
const repoRoot = "../../.."

func loadClean(t *testing.T, label, content string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "hitspec.yaml")
	write(t, path, content)
	got, err := Load(path)
	if err != nil {
		t.Errorf("%s: does not load: %v", label, err)
		return
	}
	if len(got.Warnings) > 0 {
		t.Errorf("%s: unexpected warnings: %v", label, got.Warnings)
	}
}

var yamlFence = regexp.MustCompile("(?s)```yaml[^\n]*\n(.*?)```")

// Every yaml block of the configuration reference is a valid hitspec.yaml.
func TestConfigurationDocBlocksLoadCleanly(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(repoRoot, "apps/docs/reference/configuration.mdx"))
	if err != nil {
		t.Skipf("docs not available: %v", err)
	}
	blocks := yamlFence.FindAllStringSubmatch(string(data), -1)
	if len(blocks) == 0 {
		t.Fatal("no yaml blocks found in configuration.mdx")
	}
	for i, b := range blocks {
		loadClean(t, fmt.Sprintf("configuration.mdx block %d", i+1), b[1])
	}
}

// A fully populated Config marshals (as Settings, PUT /config and the stress
// profile editor write it) to a file that loads without warnings.
func TestSavedConfigLoadsCleanly(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Retries, cfg.RetryDelay, cfg.MaxRedirects = 2, 500, 5
	cfg.Proxy, cfg.OutputDir, cfg.Concurrency = "http://p:8080", "out", 3
	cfg.Parallel, cfg.Bail, cfg.Verbose, cfg.NoColor = BoolPtr(true), BoolPtr(true), BoolPtr(true), BoolPtr(true)
	cfg.Headers = map[string]string{"X-A": "1"}
	cfg.Reporters = []string{"console"}
	cfg.Environments = map[string]map[string]any{"dev": {"baseUrl": "http://x"}}
	cfg.Stress = &StressConfig{Profiles: map[string]*StressProfile{"p": {Duration: "5s", Rate: 1, VUs: 1, MaxVUs: 2, ThinkTime: "1s", RampUp: "1s", Thresholds: map[string]string{"p95": "1s"}}}}
	path := filepath.Join(t.TempDir(), "hitspec.yaml")
	if err := cfg.SaveConfig(path); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(path)
	loadClean(t, "SaveConfig output", string(b))
}

// Every hitspec.yaml / hitspec.yml checked into the repository (fixtures,
// testdata, e2e) loads without warnings.
func TestRepositoryConfigsLoadCleanly(t *testing.T) {
	var found int
	err := filepath.WalkDir(repoRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			switch d.Name() {
			case "node_modules", ".git", ".github":
				return filepath.SkipDir
			}
			return nil
		}
		if d.Name() != "hitspec.yaml" && d.Name() != "hitspec.yml" {
			return nil
		}
		found++
		b, rerr := os.ReadFile(path)
		if rerr != nil {
			t.Error(rerr)
			return nil
		}
		// Fixtures carry placeholders (__PORT__) in values only, never keys.
		var probe yaml.Node
		if yerr := yaml.Unmarshal(b, &probe); yerr != nil {
			t.Errorf("%s: %v", path, yerr)
			return nil
		}
		loadClean(t, strings.TrimPrefix(path, repoRoot+"/"), string(b))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if found == 0 {
		t.Fatal("found no hitspec.yaml in the repository; the walk is broken")
	}
}
