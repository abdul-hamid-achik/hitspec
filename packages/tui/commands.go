package tui

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"

	"github.com/abdul-hamid-achik/hitspec/packages/clientmgr"
)

func loadFilesCmd(ctx context.Context, mgr *clientmgr.Manager) tea.Cmd {
	return func() tea.Msg {
		ws, err := mgr.Workspace(ctx)
		if err != nil {
			return filesLoadedMsg{err: err}
		}
		files, err := mgr.ListFiles(ctx)
		return filesLoadedMsg{workspace: ws, files: files, err: err}
	}
}

func loadFileCmd(ctx context.Context, mgr *clientmgr.Manager, path string) tea.Cmd {
	return func() tea.Msg {
		raw, err := mgr.ReadFile(ctx, path)
		if err != nil {
			return fileLoadedMsg{path: path, err: err}
		}
		parsed, err := mgr.GetFile(ctx, path)
		return fileLoadedMsg{path: path, raw: raw, parsed: parsed, err: err}
	}
}

func waitEventCmd(ch <-chan clientmgr.Event) tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-ch
		if !ok {
			return nil
		}
		return managerEventMsg(ev)
	}
}

func loadHistoryCmd(ctx context.Context, mgr *clientmgr.Manager) tea.Cmd {
	return func() tea.Msg {
		h, err := mgr.ListRuns(ctx, 30, 0)
		return historyMsg{history: h, err: err}
	}
}

func loadCookiesCmd(ctx context.Context, mgr *clientmgr.Manager) tea.Cmd {
	return func() tea.Msg {
		c, err := mgr.ListCookies(ctx)
		return cookiesMsg{cookies: c, err: err}
	}
}

func loadConfigCmd(ctx context.Context, mgr *clientmgr.Manager) tea.Cmd {
	return func() tea.Msg {
		cfg, err := mgr.GetConfig(ctx)
		if err != nil {
			return configMsg{err: err}
		}
		envs, err := mgr.ListEnvironments(ctx)
		return configMsg{config: cfg, envs: envs, err: err}
	}
}

// selectEnvCmd activates an environment and returns refreshed config, env list,
// and workspace so the UI (topbar, settings, switcher) updates atomically.
func selectEnvCmd(ctx context.Context, mgr *clientmgr.Manager, name string) tea.Cmd {
	return func() tea.Msg {
		if err := mgr.SelectEnvironment(ctx, name); err != nil {
			return envSelectedMsg{name: name, err: err}
		}
		cfg, err := mgr.GetConfig(ctx)
		if err != nil {
			return envSelectedMsg{name: name, err: err}
		}
		envs, err := mgr.ListEnvironments(ctx)
		if err != nil {
			return envSelectedMsg{name: name, err: err}
		}
		ws, err := mgr.Workspace(ctx)
		return envSelectedMsg{name: name, config: cfg, envs: envs, workspace: ws, err: err}
	}
}

// setEnvVar sets or updates one variable on an environment via read-modify-write
// so the other variables are preserved (PutEnvironment replaces the whole map).
func setEnvVar(ctx context.Context, mgr *clientmgr.Manager, name, key, value string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("environment name is required")
	}
	// Copy the existing variables rather than aliasing the Manager's internal map
	// (GetEnvironment returns it by reference), so we never mutate manager state
	// outside its lock or before PutEnvironment validates the write.
	vars := map[string]any{}
	if env, err := mgr.GetEnvironment(ctx, name); err == nil {
		for k, v := range env.Variables {
			vars[k] = v
		}
	}
	vars[key] = value
	_, err := mgr.PutEnvironment(ctx, name, vars)
	return err
}

func startStressCmd(ctx context.Context, mgr *clientmgr.Manager, file string) tea.Cmd {
	return func() tea.Msg {
		files := []string{}
		if file != "" {
			files = append(files, file)
		}
		err := mgr.StartStress(ctx, clientmgr.StressStartReq{Files: files, Duration: "30s", Rate: 5})
		return simpleMsg{kind: "stress started", err: err}
	}
}

func startMockCmd(ctx context.Context, mgr *clientmgr.Manager, file string) tea.Cmd {
	return func() tea.Msg {
		files := []string{}
		if file != "" {
			files = append(files, file)
		}
		_, err := mgr.StartMock(ctx, clientmgr.MockStartReq{Files: files, Port: 3000})
		return simpleMsg{kind: "mock started", err: err}
	}
}

func stopMockCmd(ctx context.Context, mgr *clientmgr.Manager) tea.Cmd {
	return func() tea.Msg {
		err := mgr.StopMock(ctx)
		return simpleMsg{kind: "mock stopped", err: err}
	}
}

func exportRecordCmd(ctx context.Context, mgr *clientmgr.Manager) tea.Cmd {
	return func() tea.Msg {
		content, err := mgr.ExportRecordings(ctx)
		return previewMsg{title: "recordings exported", content: content, err: err}
	}
}

func importCmd(ctx context.Context, mgr *clientmgr.Manager, format, input, baseURL string) tea.Cmd {
	return func() tea.Msg {
		format = strings.ToLower(strings.TrimSpace(format))
		var result clientmgr.ImportResultDTO
		var err error
		switch format {
		case "curl":
			result, err = mgr.ImportCurl(ctx, clientmgr.ImportCurlReq{Command: input})
		case "insomnia":
			req := clientmgr.ImportInsomniaReq{Data: input}
			if !strings.HasPrefix(strings.TrimSpace(input), "{") {
				req = clientmgr.ImportInsomniaReq{FilePath: input}
			}
			result, err = mgr.ImportInsomnia(ctx, req)
		case "openapi":
			result, err = mgr.ImportOpenAPI(ctx, clientmgr.ImportOpenAPIReq{SpecPath: input, BaseURL: baseURL})
		case "postman":
			req := clientmgr.ImportPostmanReq{Data: input}
			if !strings.HasPrefix(strings.TrimSpace(input), "{") {
				req = clientmgr.ImportPostmanReq{FilePath: input}
			}
			result, err = mgr.ImportPostman(ctx, req)
		default:
			err = fmt.Errorf("unsupported import format: %s", format)
		}
		if err != nil {
			return importMsg{err: err}
		}
		path := fmt.Sprintf("imported-%d.http", time.Now().Unix())
		parsed, err := mgr.CreateFile(ctx, path, result.Content)
		if err != nil {
			return importMsg{err: err}
		}
		return importMsg{path: path, raw: result.Content, parsed: parsed}
	}
}

func buildCommandItems() []list.Item {
	commands := []commandItem{
		{"Run active request", "Execute selected request", "run-request"},
		{"Run file", "Execute every request in the selected file", "run-file"},
		{"Save file", "Persist inline source changes", "save"},
		{"Edit source", "Focus the inline textarea editor", "edit"},
		{"New file", "Create a scratch .http file", "new-file"},
		{"Generate sample project", "Create hitspec.yaml + example.http", "scaffold"},
		{"Rename file", "Rename/move the selected file", "rename-file"},
		{"Duplicate file", "Copy the selected file to a new path", "duplicate-file"},
		{"Delete file", "Delete the selected file", "delete-file"},
		{"Refresh workspace", "Rescan files and config", "refresh"},
		{"Copy request as curl", "Copy the selected request as a curl command", "copy-curl"},
		{"Copy request as HTTPie", "Copy the selected request as an HTTPie command", "copy-httpie"},
		{"Copy request as Python", "Copy the selected request as Python requests code", "copy-python"},
		{"Copy request as fetch", "Copy the selected request as a JS fetch snippet", "copy-fetch"},
		{"Copy request as Go", "Copy the selected request as Go net/http code", "copy-go"},
		{"Copy response body", "Copy the last response body to the clipboard", "copy-response"},
		{"Switch environment", "Pick the active environment (ctrl+e)", "env-switch"},
		{"Switch theme", "Pick a color theme (ctrl+t)", "theme"},
		{"Search requests", "Find requests across the workspace (ctrl+f)", "search"},
		{"Quick request (ad-hoc)", "Run a one-off request without saving a file", "adhoc"},
		{"Start stress", "30s at 5 RPS on selected file", "stress-start"},
		{"Stop stress", "Stop the running stress test", "stress-stop"},
		{"Start mock", "Start mock server on :3000", "mock-start"},
		{"Stop mock", "Stop the running mock server", "mock-stop"},
		{"Export recordings", "Preview recorded traffic as .http", "record-export"},
		{"Clear recordings", "Discard all captured proxy traffic", "record-clear"},
		{"Clear history", "Delete all persistent run history", "history-clear"},
		{"History", "Open persistent run history", "history"},
		{"Cookies", "Open local cookie store", "cookies"},
		{"Settings", "Open config and environments", "settings"},
	}
	for _, name := range screenNames {
		commands = append(commands, commandItem{"Go to " + name, "Navigate to screen", name})
	}
	items := make([]list.Item, 0, len(commands))
	for _, c := range commands {
		items = append(items, c)
	}
	return items
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func atoi(s string, fallback int) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		return fallback
	}
	return n
}

// containsFile reports whether rel is among the files by relative path.
func containsFile(files []clientmgr.FileInfoDTO, rel string) bool {
	for _, f := range files {
		if f.RelativePath == rel {
			return true
		}
	}
	return false
}

func selectedFiles(path string) []string {
	if path == "" {
		return nil
	}
	return []string{path}
}

func defaultString(s, fallback string) string {
	if strings.TrimSpace(s) == "" {
		return fallback
	}
	return s
}
