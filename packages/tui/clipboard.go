package tui

import (
	"context"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/atotto/clipboard"

	"github.com/abdul-hamid-achik/hitspec/v2/packages/clientmgr"
)

// copyToClipboard writes text to the system clipboard. It is best-effort: on
// headless machines without a clipboard utility it returns an error that callers
// surface as a soft notice rather than a failure.
func copyToClipboard(text string) error {
	return clipboard.WriteAll(text)
}

// exportText renders the selected request (or every request in a file when the
// selection is empty) to a client format using the manager's exporter. It is a
// pure function so it can be unit-tested without a clipboard.
func exportText(ctx context.Context, mgr *clientmgr.Manager, req clientmgr.ExportSelectionReq) (string, error) {
	if req.File == "" {
		return "", fmt.Errorf("no file selected")
	}
	res, err := mgr.ExportSelection(ctx, req)
	if err != nil {
		return "", err
	}
	if len(res.Commands) == 0 {
		return "", fmt.Errorf("nothing to export from %s", req.File)
	}
	return strings.Join(res.Commands, "\n\n"), nil
}

// exportSelected exports the highlighted request. It exports the request as it is
// in the editor (unsaved edits included), by position so an untitled request
// works, with {{variables}} resolved from the active environment.
func (m *model) exportSelected(format string) tea.Cmd {
	name, index := m.currentRequestSelection()
	req := clientmgr.ExportSelectionReq{File: m.selected, RequestName: name, RequestIndex: &index, Format: format}
	if m.bufferModified() {
		req.Source = m.source.Value()
	}
	return exportCmd(m.ctx, m.mgr, req)
}

// exportCmd exports a request to a client format, copies it to the clipboard
// (best-effort), and surfaces the rendered snippet so the user can inspect it.
func exportCmd(ctx context.Context, mgr *clientmgr.Manager, req clientmgr.ExportSelectionReq) tea.Cmd {
	return func() tea.Msg {
		text, err := exportText(ctx, mgr, req)
		if err != nil {
			return copyMsg{err: err}
		}
		title := fmt.Sprintf("rendered as %s (clipboard unavailable)", req.Format)
		if copyToClipboard(text) == nil {
			title = fmt.Sprintf("copied as %s to clipboard", req.Format)
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
