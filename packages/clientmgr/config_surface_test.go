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

			env, configEnvs := m.envSnapshot("")
			cfg := m.runnerConfig(env, configEnvs)
			table["studio.execute"][sc.Name] = cfgsurface.FromRunner(cfg)
			table["studio.adhoc"][sc.Name] = cfgsurface.FromRunner(cfg)

			// Stress and contract build their clients with no options.
			table["studio.stress"][sc.Name] = cfgsurface.Bare()
			table["studio.contract"][sc.Name] = cfgsurface.Bare()
		})
	}
	cfgsurface.Check(t, filepath.Join("testdata", "config_surfaces.golden.json"), table)
}
