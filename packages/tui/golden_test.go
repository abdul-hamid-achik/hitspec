package tui

import (
	"context"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/golden"

	"github.com/abdul-hamid-achik/hitspec/packages/clientmgr"
)

// plain strips ANSI so golden files capture layout + text deterministically,
// independent of color (lipgloss v2 emits truecolor at Render; chroma adds 256
// color — both are stripped here).
func plain(s string) string { return ansi.Strip(s) }

// goldenModel builds a deterministic model: no color, no spinner/progress, fixed
// size. The response viewer is rebuilt with color off too.
func goldenModel(t *testing.T, w, h int) model {
	t.Helper()
	m := newModel(context.Background(), newTestManager(t), Options{})
	m.color = false
	m.respView = newResponseViewer(m.styles, false)
	m.width, m.height = w, h
	m.resize()
	return m
}

func TestGoldenWorkspaceWide(t *testing.T) {
	m := goldenModel(t, 130, 40)
	m.workspace = clientmgr.WorkspaceDTO{Root: "/ws", Environment: "dev", TotalRequests: 1}
	m.files = []clientmgr.FileInfoDTO{{RelativePath: "api.http", Name: "api.http", RequestCount: 1}}
	m.selected = "api.http"
	m.parsed = &clientmgr.ParsedFileDTO{Requests: []clientmgr.RequestDTO{
		{Name: "Ping", Method: "GET", URL: "https://example.com/api"},
	}}
	m.refreshFileList()
	m.refreshRequestTables()
	m.lastResult = sampleResult()
	m.refreshResultViews()
	m.focus = focusResponse
	golden.RequireEqual(t, plain(m.View().Content))
}

func TestGoldenWorkspaceNarrow(t *testing.T) {
	m := goldenModel(t, 60, 24)
	m.workspace = clientmgr.WorkspaceDTO{Environment: "dev"}
	m.files = []clientmgr.FileInfoDTO{{RelativePath: "api.http", Name: "api.http", RequestCount: 1}}
	m.selected = "api.http"
	m.refreshFileList()
	golden.RequireEqual(t, plain(m.View().Content))
}

// TestGoldenWorkspaceNarrowResponse exercises the narrow single-panel layout
// with the response pane focused (and a result loaded), covering the focus-
// switched content branch and the tab bar inside a narrow panel.
func TestGoldenWorkspaceNarrowResponse(t *testing.T) {
	m := goldenModel(t, 60, 24)
	m.workspace = clientmgr.WorkspaceDTO{Environment: "dev"}
	m.files = []clientmgr.FileInfoDTO{{RelativePath: "api.http", Name: "api.http", RequestCount: 1}}
	m.selected = "api.http"
	m.refreshFileList()
	m.lastResult = sampleResult()
	m.refreshResultViews()
	m.focus = focusResponse
	golden.RequireEqual(t, plain(m.View().Content))
}

func TestGoldenWelcomeFresh(t *testing.T) {
	m := goldenModel(t, 100, 30)
	m.workspace = clientmgr.WorkspaceDTO{Environment: "dev", HasConfig: false}
	m.files = nil
	m.refreshFileList()
	golden.RequireEqual(t, plain(m.View().Content))
}

func TestGoldenWelcomeHasConfig(t *testing.T) {
	m := goldenModel(t, 100, 30)
	m.workspace = clientmgr.WorkspaceDTO{Environment: "dev", HasConfig: true}
	m.files = nil
	m.refreshFileList()
	golden.RequireEqual(t, plain(m.View().Content))
}

func TestGoldenHelpOverlay(t *testing.T) {
	m := goldenModel(t, 100, 30)
	m.workspace = clientmgr.WorkspaceDTO{Environment: "dev"}
	m.files = []clientmgr.FileInfoDTO{{RelativePath: "api.http", Name: "api.http", RequestCount: 1}}
	m.selected = "api.http"
	m.refreshFileList()
	m.showHelp = true
	golden.RequireEqual(t, plain(m.View().Content))
}

func TestGoldenThemePicker(t *testing.T) {
	m := goldenModel(t, 100, 30)
	m.workspace = clientmgr.WorkspaceDTO{Environment: "dev"}
	m.files = []clientmgr.FileInfoDTO{{RelativePath: "api.http", Name: "api.http", RequestCount: 1}}
	m.selected = "api.http"
	m.refreshFileList()
	m.themeOpen = true
	m.themeList.SetItems(buildThemeItems(m.theme))
	golden.RequireEqual(t, plain(m.View().Content))
}

func TestGoldenResponseTabs(t *testing.T) {
	rv := newResponseViewer(newStyles(defaultPalette()), false)
	rv.setResult(sampleResult())
	for _, tab := range []responseTab{respBody, respHeaders, respAssertions, respTiming, respCaptures} {
		rv.tab = tab
		t.Run(responseTabNames[tab], func(t *testing.T) {
			golden.RequireEqual(t, plain(rv.tabContent()))
		})
	}
}

func TestGoldenConfirmDialog(t *testing.T) {
	c := &confirmState{title: "Delete file?", body: "doomed.http"}
	golden.RequireEqual(t, plain(c.view(newStyles(defaultPalette()))))
}

func TestGoldenPrompt(t *testing.T) {
	p := newPrompt("Rename file", "new/path.http", "a.http", nil)
	golden.RequireEqual(t, plain(p.view(newStyles(defaultPalette()))))
}

func TestGoldenSearchOverlay(t *testing.T) {
	m := goldenModel(t, 100, 30)
	m.searchOpen = true
	m.searchInput.SetValue("users")
	m.searchResults = []clientmgr.SearchResultDTO{
		{File: "api.http", RequestName: "List users", Method: "GET", URL: "https://x/users"},
		{File: "api.http", RequestName: "", Method: "POST", URL: "https://x/users", Line: 12},
	}
	m.refreshSearchList()
	body := lipgloss.JoinVertical(lipgloss.Left, m.searchInput.View(), "", m.searchList.View())
	golden.RequireEqual(t, plain(body))
}

func TestGoldenToasts(t *testing.T) {
	c := newToastCenter()
	c, _ = c.push(toastInfo, "info message")
	c, _ = c.push(toastSuccess, "saved file")
	c, _ = c.push(toastError, "request failed")
	golden.RequireEqual(t, plain(c.view(newStyles(defaultPalette()))))
}

func TestGoldenStressResult(t *testing.T) {
	m := goldenModel(t, 100, 30)
	m.stressResult = &clientmgr.StressResultDTO{
		DurationMs: 30000, Total: 1000, Success: 990, Errors: 10, RPS: 33.3,
		SuccessRate: 0.99, ErrorRate: 0.01, P50Ms: 12, P95Ms: 45, P99Ms: 80,
		MinMs: 5, MaxMs: 120, MeanMs: 15,
		Breakdown: []clientmgr.StressRequestBreakdownDTO{
			{Name: "login", Total: 500, Success: 495, Errors: 5, P95Ms: 40},
			{Name: "fetch", Total: 500, Success: 495, Errors: 5, P95Ms: 50},
		},
	}
	golden.RequireEqual(t, plain(m.stressResultContent()))
}

func TestGoldenSettingsContent(t *testing.T) {
	m := goldenModel(t, 100, 30)
	m.config = clientmgr.ConfigDTO{DefaultEnvironment: "dev", Timeout: 30000, Retries: 2, Concurrency: 4}
	m.workspace = clientmgr.WorkspaceDTO{Environment: "dev"}
	m.envs = []clientmgr.EnvironmentDTO{
		{Name: "dev", Variables: map[string]any{"baseUrl": "http://localhost:3000"}},
		{Name: "prod", Variables: map[string]any{"baseUrl": "https://api.example.com"}},
	}
	golden.RequireEqual(t, plain(m.settingsContent()))
}

// TestGoldenSettingsScreen renders the whole settings screen (form + config
// dump) so the editable form fields can never silently disappear again — the
// configMsg handler used to overwrite the preview with the form-less content.
func TestGoldenSettingsScreen(t *testing.T) {
	m := goldenModel(t, 100, 30)
	m.config = clientmgr.ConfigDTO{DefaultEnvironment: "dev", Timeout: 30000, Retries: 2, Concurrency: 4}
	m.workspace = clientmgr.WorkspaceDTO{Environment: "dev"}
	m.envs = []clientmgr.EnvironmentDTO{
		{Name: "dev", Variables: map[string]any{"baseUrl": "http://localhost:3000"}},
	}
	m.setScreen(screenSettings)
	golden.RequireEqual(t, plain(m.View().Content))
}

// TestGoldenStressScreen renders the full stress screen so its bordered panel,
// form, and footer hint stay aligned within the body.
func TestGoldenStressScreen(t *testing.T) {
	m := goldenModel(t, 100, 30)
	m.workspace = clientmgr.WorkspaceDTO{Environment: "dev"}
	m.selected = "api.http"
	m.setScreen(screenStress)
	golden.RequireEqual(t, plain(m.View().Content))
}

// TestGoldenStressRunning renders the live metrics view shown mid-test (the
// progress bar + RPS/percentile readout), covering stressContent's running arm.
func TestGoldenStressRunning(t *testing.T) {
	m := goldenModel(t, 100, 30)
	m.workspace = clientmgr.WorkspaceDTO{Environment: "dev"}
	m.selected = "api.http"
	m.setScreen(screenStress)
	stats := clientmgr.StressStatsDTO{
		Total: 450, Success: 442, Errors: 8, RPS: 30.5,
		P50Ms: 12, P95Ms: 48, P99Ms: 90, ErrorRate: 0.018, ActiveVUs: 5,
	}
	m.stress = clientmgr.StressStatusDTO{Running: true, Elapsed: 15.0, Stats: &stats}
	m.preview.SetContent(m.secondaryContent())
	golden.RequireEqual(t, plain(m.View().Content))
}

// TestGoldenImportScreen and TestGoldenCookiesScreen render the remaining
// form-backed secondary screens through the full View so their panel, form, and
// footer stay aligned under the shared workspace geometry.
func TestGoldenImportScreen(t *testing.T) {
	m := goldenModel(t, 100, 30)
	m.workspace = clientmgr.WorkspaceDTO{Environment: "dev"}
	m.setScreen(screenImport)
	golden.RequireEqual(t, plain(m.View().Content))
}

func TestGoldenCookiesScreen(t *testing.T) {
	m := goldenModel(t, 100, 30)
	m.workspace = clientmgr.WorkspaceDTO{Environment: "dev"}
	m.cookies = []clientmgr.CookieDTO{
		{Domain: "example.com", Path: "/", Name: "session", Value: "abc123"},
	}
	m.setScreen(screenCookies)
	m.preview.SetContent(m.secondaryContent())
	golden.RequireEqual(t, plain(m.View().Content))
}

// TestGoldenMockScreen / RecordScreen / ContractScreen / HistoryScreen render
// the populated state of each remaining secondary screen so the content
// renderers (mockContent, recordContent, contractContent) and the history list
// stay aligned within the bordered panel and keep their layout under test.
func TestGoldenMockScreen(t *testing.T) {
	m := goldenModel(t, 100, 30)
	m.workspace = clientmgr.WorkspaceDTO{Environment: "dev"}
	m.setScreen(screenMock)
	m.mock = clientmgr.MockStatusDTO{
		Running: true, Port: 3000,
		Routes: []clientmgr.MockRouteDTO{
			{Method: "GET", Path: "/health", StatusCode: 200, ContentType: "application/json"},
			{Method: "POST", Path: "/users", StatusCode: 201, ContentType: "application/json"},
		},
	}
	m.preview.SetContent(m.secondaryContent())
	golden.RequireEqual(t, plain(m.View().Content))
}

func TestGoldenRecordScreen(t *testing.T) {
	m := goldenModel(t, 100, 30)
	m.workspace = clientmgr.WorkspaceDTO{Environment: "dev"}
	m.setScreen(screenRecord)
	m.record = clientmgr.RecordStatusDTO{
		Running: true, TargetURL: "http://localhost:3000", Port: 8081, Count: 2,
		Recordings: []clientmgr.RecordingDTO{
			{Method: "GET", Path: "/health", StatusCode: 200, Duration: 12},
			{Method: "POST", Path: "/users", StatusCode: 201, Duration: 34},
		},
	}
	m.preview.SetContent(m.secondaryContent())
	golden.RequireEqual(t, plain(m.View().Content))
}

func TestGoldenContractScreen(t *testing.T) {
	m := goldenModel(t, 100, 30)
	m.workspace = clientmgr.WorkspaceDTO{Environment: "dev"}
	m.setScreen(screenContract)
	m.contracts = []clientmgr.ContractResultDTO{{
		File: "api.http", Passed: 1, Failed: 1, Skipped: 0, Duration: 120,
		Results: []clientmgr.ContractInteractionDTO{
			{Name: "health", Passed: true, Duration: 40},
			{Name: "createUser", Passed: false, Duration: 80, State: "seeded", Error: "status 500"},
		},
	}}
	m.preview.SetContent(m.secondaryContent())
	golden.RequireEqual(t, plain(m.View().Content))
}

func TestGoldenHistoryScreen(t *testing.T) {
	m := goldenModel(t, 100, 30)
	m.workspace = clientmgr.WorkspaceDTO{Environment: "dev"}
	m.history = clientmgr.HistoryListDTO{
		Total: 2,
		Runs: []clientmgr.HistoryRunDTO{
			{ID: 2, FilePath: "api.http", StartedAt: "2026-06-14T10:00:00Z", Passed: 3, Failed: 0, Skipped: 0, DurationMs: 1200},
			{ID: 1, FilePath: "auth.http", StartedAt: "2026-06-14T09:00:00Z", Passed: 1, Failed: 2, Skipped: 1, DurationMs: 800},
		},
	}
	m.refreshHistoryList()
	m.screen = screenHistory
	golden.RequireEqual(t, plain(m.View().Content))
}

func TestGoldenHistoryDetail(t *testing.T) {
	m := goldenModel(t, 100, 30)
	m.historyDetail = &clientmgr.HistoryRunDTO{
		ID: 7, FilePath: "api.http", Environment: "dev",
		StartedAt: "2026-06-14T10:00:00Z", DurationMs: 1200, Passed: 1, Total: 1,
		Results: []clientmgr.HistoryResultDTO{{
			RequestName: "Ping", Method: "GET", URL: "https://example.com/api",
			StatusCode: 200, DurationMs: 42, Passed: true,
			Assertions: []clientmgr.HistoryAssertionDTO{
				{Subject: "status", Operator: "==", Expected: "200", Passed: true},
			},
		}},
	}
	golden.RequireEqual(t, plain(m.historyDetailContent()))
}
