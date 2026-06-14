package tui

import (
	"context"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/atotto/clipboard"

	"github.com/abdul-hamid-achik/hitspec/packages/clientmgr"
)

// copyToClipboard writes text to the system clipboard. It is best-effort: on
// headless machines without a clipboard utility it returns an error that callers
// surface as a soft notice rather than a failure.
func copyToClipboard(text string) error {
	return clipboard.WriteAll(text)
}

// exportText renders a request (or every request in a file when requestName is
// empty) to the given client format using the manager's exporter. It is a pure
// function so it can be unit-tested without a clipboard.
func exportText(ctx context.Context, mgr *clientmgr.Manager, file, requestName, format string) (string, error) {
	if file == "" {
		return "", fmt.Errorf("no file selected")
	}
	res, err := mgr.Export(ctx, clientmgr.ExportReq{File: file, RequestName: requestName, Format: format})
	if err != nil {
		return "", err
	}
	if len(res.Commands) == 0 {
		return "", fmt.Errorf("nothing to export from %s", file)
	}
	return strings.Join(res.Commands, "\n\n"), nil
}

// exportCmd exports a request to a client format, copies it to the clipboard
// (best-effort), and surfaces the rendered snippet so the user can inspect it.
func exportCmd(ctx context.Context, mgr *clientmgr.Manager, file, requestName, format string) tea.Cmd {
	return func() tea.Msg {
		text, err := exportText(ctx, mgr, file, requestName, format)
		if err != nil {
			return copyMsg{err: err}
		}
		title := fmt.Sprintf("rendered as %s (clipboard unavailable)", format)
		if copyToClipboard(text) == nil {
			title = fmt.Sprintf("copied as %s to clipboard", format)
		}
		return copyMsg{title: title, content: text}
	}
}

// copyTextCmd copies an already-available string (e.g. a response body) to the
// clipboard without touching the manager.
func copyTextCmd(text, label string) tea.Cmd {
	return func() tea.Msg {
		if strings.TrimSpace(text) == "" {
			return copyMsg{err: fmt.Errorf("nothing to copy")}
		}
		if err := copyToClipboard(text); err != nil {
			return copyMsg{title: label + " (clipboard unavailable)", content: text}
		}
		return copyMsg{title: label + " copied to clipboard"}
	}
}
