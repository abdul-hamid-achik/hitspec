package clientmgr

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/abdul-hamid-achik/hitspec/v2/packages/core/config"
)

// The scaffold written by `hitspec init` and the in-app "generate sample
// project" action loads without a single unknown-key warning.
func TestSampleConfigLoadsWithoutWarnings(t *testing.T) {
	path := filepath.Join(t.TempDir(), SampleConfigFile)
	if err := os.WriteFile(path, []byte(SampleConfigYAML), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Warnings) != 0 {
		t.Fatalf("scaffold warns: %v", got.Warnings)
	}
}
