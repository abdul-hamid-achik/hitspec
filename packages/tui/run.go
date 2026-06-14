// Package tui implements the native terminal UI (Charm Bubble Tea v2) for the
// hitspec API client manager, rendering the in-process clientmgr.Manager.
package tui

import (
	"context"
	"errors"

	tea "charm.land/bubbletea/v2"

	"github.com/abdul-hamid-achik/hitspec/packages/clientmgr"
)

// Options configures the TUI runtime.
type Options struct {
	Mouse bool
	Theme string // theme name (case-insensitive); empty uses the default
}

// Run starts the native API Client Manager TUI.
func Run(ctx context.Context, mgr *clientmgr.Manager, opts Options) error {
	if ctx == nil {
		ctx = context.Background()
	}
	model := newModel(ctx, mgr, opts)
	_, err := tea.NewProgram(model, tea.WithContext(ctx), tea.WithFPS(60)).Run()
	if errors.Is(err, tea.ErrInterrupted) {
		return nil
	}
	return err
}
