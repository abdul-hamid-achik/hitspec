package tui

import (
	"context"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/abdul-hamid-achik/hitspec/packages/clientmgr"
)

// renameFileCmd renames a file then opens the result.
func renameFileCmd(ctx context.Context, mgr *clientmgr.Manager, oldPath, newPath, action string) tea.Cmd {
	return func() tea.Msg {
		parsed, err := mgr.RenameFile(ctx, oldPath, newPath)
		if err != nil {
			return fileRenamedMsg{path: newPath, action: action, err: err}
		}
		raw, err := mgr.ReadFile(ctx, newPath)
		if err != nil {
			return fileRenamedMsg{path: newPath, action: action, err: err}
		}
		return fileRenamedMsg{path: newPath, raw: raw, parsed: parsed, action: action}
	}
}

// copyFileCmd duplicates a file then opens the copy.
func copyFileCmd(ctx context.Context, mgr *clientmgr.Manager, srcPath, dstPath, action string) tea.Cmd {
	return func() tea.Msg {
		parsed, err := mgr.CopyFile(ctx, srcPath, dstPath)
		if err != nil {
			return fileRenamedMsg{path: dstPath, action: action, err: err}
		}
		raw, err := mgr.ReadFile(ctx, dstPath)
		if err != nil {
			return fileRenamedMsg{path: dstPath, action: action, err: err}
		}
		return fileRenamedMsg{path: dstPath, raw: raw, parsed: parsed, action: action}
	}
}

// suggestCopyName proposes "<name>-copy<ext>" as the default duplicate path.
func suggestCopyName(path string) string {
	ext := filepath.Ext(path)
	base := strings.TrimSuffix(path, ext)
	if ext == "" {
		ext = ".http"
	}
	return base + "-copy" + ext
}
