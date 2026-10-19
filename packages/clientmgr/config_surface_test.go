package clientmgr

import (
	"path/filepath"
	"testing"

	"github.com/abdul-hamid-achik/hitspec/v2/internal/cfgsurface"
)

// TestConfigSurfaces records the settings each studio path (execute, run,
// ad-hoc, stress, contract) ends up using for the shared hitspec.yaml / env
// scenarios. The studio takes no run flags, so the flags scenario matches the
// env one. See internal/cfgsurface.
func TestConfigSurfaces(t *testing.T) {
	table := map[string]map[string]cfgsurface.Settings{
		"studio.execute":  {},
		"studio.adhoc":    {},
		"studio.stress":   {},
		"studio.contract": {},
	}
	for _, sc := range cfgsurface.Scenarios() {
		t.Run(sc.Name, func(t *testing.T) {
			dir := cfgsurface.Setup(t, sc)
			m := New(WithWorkDir(dir), WithWatch(false), WithHistoryDBPath(filepath.Join(dir, "h.db")))
			defer func() { _ = m.Close() }()

			cfg, _ := m.runSettings("")
			run := cfgsurface.FromRunner(cfg)
			table["studio.execute"][sc.Name] = run
			table["studio.adhoc"][sc.Name] = run

			// Stress and contract build their clients from the same options.
			table["studio.stress"][sc.Name] = cfgsurface.ClientOnly(run)
			table["studio.contract"][sc.Name] = cfgsurface.ClientOnly(run)
		})
	}
	cfgsurface.Check(t, filepath.Join("testdata", "config_surfaces.golden.json"), table)
}
