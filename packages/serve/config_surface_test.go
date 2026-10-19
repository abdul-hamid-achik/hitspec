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

			cfg, _ := s.runSettings("")
			run := cfgsurface.FromRunner(cfg)
			table["serve.execute"][sc.Name] = run
			table["serve.run"][sc.Name] = run

			// Stress and contract build their clients from the same options.
			table["serve.stress"][sc.Name] = cfgsurface.ClientOnly(run)
			table["serve.contract"][sc.Name] = cfgsurface.ClientOnly(run)
		})
	}
	cfgsurface.Check(t, filepath.Join("testdata", "config_surfaces.golden.json"), table)
}
