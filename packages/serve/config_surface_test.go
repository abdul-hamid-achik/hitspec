package serve

import (
	"path/filepath"
	"testing"

	"github.com/abdul-hamid-achik/hitspec/v2/internal/cfgsurface"
)

// TestConfigSurfaces records the settings each serve --api-only path (execute,
// run, stress, contract) ends up using for the shared hitspec.yaml / env
// scenarios. The server takes no run flags, so the flags scenario matches the
// env one. See internal/cfgsurface.
func TestConfigSurfaces(t *testing.T) {
	table := map[string]map[string]cfgsurface.Settings{
		"serve.execute":  {},
		"serve.run":      {},
		"serve.stress":   {},
		"serve.contract": {},
	}
	for _, sc := range cfgsurface.Scenarios() {
		t.Run(sc.Name, func(t *testing.T) {
			dir := cfgsurface.Setup(t, sc)
			s := NewServer(WithWorkDir(dir), WithWatch(false), WithHistoryDBPath(filepath.Join(dir, "h.db")))
			t.Cleanup(func() {
				if s.historyStore != nil {
					_ = s.historyStore.Close()
				}
			})

			s.configMu.RLock()
			env := s.config.Env
			configEnvs := s.getConfigEnvsLocked()
			s.configMu.RUnlock()
			cfg := s.runnerConfig(env, configEnvs)
			table["serve.execute"][sc.Name] = cfgsurface.FromRunner(cfg)
			table["serve.run"][sc.Name] = cfgsurface.FromRunner(cfg)

			// Stress and contract build their clients with no options.
			table["serve.stress"][sc.Name] = cfgsurface.Bare()
			table["serve.contract"][sc.Name] = cfgsurface.Bare()
		})
	}
	cfgsurface.Check(t, filepath.Join("testdata", "config_surfaces.golden.json"), table)
}
