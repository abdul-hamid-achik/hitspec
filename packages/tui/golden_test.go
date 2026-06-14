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
