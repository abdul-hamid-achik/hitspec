// Package tui implements the native terminal UI (Charm Bubble Tea v2) for the
// hitspec API client manager, rendering the in-process clientmgr.Manager.
package tui

import (
	"context"
	"errors"
	"io"

	tea "charm.land/bubbletea/v2"

	"github.com/abdul-hamid-achik/hitspec/v2/packages/clientmgr"
)

// Options configures the TUI runtime.
type Options struct {
	Mouse bool
	Theme string // theme name (case-insensitive); empty uses the default
	// Output is where the UI is drawn; nil means os.Stdout. A caller that has
	// redirected os.Stdout/os.Stderr to keep stray engine output off the screen
	// passes the real terminal here.
	Output io.Writer
}

// Run starts the native API Client Manager TUI.
func Run(ctx context.Context, mgr *clientmgr.Manager, opts Options) error {
	if ctx == nil {
		ctx = context.Background()
	}
	model := newModel(ctx, mgr, opts)
	progOpts := []tea.ProgramOption{tea.WithContext(ctx), tea.WithFPS(60)}
	if opts.Output != nil {
		progOpts = append(progOpts, tea.WithOutput(opts.Output))
	}
	_, err := tea.NewProgram(model, progOpts...).Run()
	if errors.Is(err, tea.ErrInterrupted) {
		return nil
	}
	return err
}
