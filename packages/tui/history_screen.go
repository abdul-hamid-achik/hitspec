package tui

import (
	"context"
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"

	"github.com/abdul-hamid-achik/hitspec/packages/clientmgr"
)

// historyItem is a persistent run shown in the interactive history list.
type historyItem struct {
	id       int64
	file     string
	started  string
	passed   int64
	failed   int64
	skipped  int64
	duration int64
}

func (i historyItem) FilterValue() string { return i.file }
func (i historyItem) Title() string       { return fmt.Sprintf("#%d  %s", i.id, i.file) }
func (i historyItem) Description() string {
	return fmt.Sprintf("%s   pass:%d fail:%d skip:%d   %.1fs",
		i.started, i.passed, i.failed, i.skipped, float64(i.duration)/1000.0)
}

// runDetailMsg carries a single run's full details (GetRun) for the detail view.
type runDetailMsg struct {
	run clientmgr.HistoryRunDTO
	err error
}

func loadRunCmd(ctx context.Context, mgr *clientmgr.Manager, id int64) tea.Cmd {
	return func() tea.Msg {
		run, err := mgr.GetRun(ctx, id)
		return runDetailMsg{run: run, err: err}
	}
}

func (m *model) refreshHistoryList() {
	items := make([]list.Item, 0, len(m.history.Runs))
	for _, r := range m.history.Runs {
		items = append(items, historyItem{
			id: r.ID, file: r.FilePath, started: r.StartedAt,
			passed: r.Passed, failed: r.Failed, skipped: r.Skipped, duration: r.DurationMs,
		})
	}
	_ = m.historyList.SetItems(items)
}

// handleHistoryKey drives the interactive history screen: a run list with
// enter→details, D→delete (confirmed), ctrl+r→refresh, esc→back.
func (m *model) handleHistoryKey(msg tea.KeyPressMsg) tea.Cmd {
	if m.historyDetailMode {
		switch {
		case key.Matches(msg, m.keys.Cancel):
			m.transitioned = true
			m.historyDetailMode = false
			return nil
		case key.Matches(msg, m.keys.Refresh):
			return loadHistoryCmd(m.ctx, m.mgr)
		}
		return nil // scroll keys fall through to the preview viewport
	}

	switch {
	case key.Matches(msg, m.keys.Refresh):
		return loadHistoryCmd(m.ctx, m.mgr)
	case key.Matches(msg, m.keys.Open), key.Matches(msg, m.keys.Confirm):
		if it, ok := m.historyList.SelectedItem().(historyItem); ok {
			m.transitioned = true
			return loadRunCmd(m.ctx, m.mgr, it.id)
		}
		return nil
	}

	if msg.String() == "D" {
		if it, ok := m.historyList.SelectedItem().(historyItem); ok {
			id := it.id
			ctx, mgr := m.ctx, m.mgr
			m.transitioned = true
			m.confirm = &confirmState{
				title: "Delete run?",
				body:  fmt.Sprintf("#%d  %s", it.id, it.file),
				action: func() tea.Msg {
					if err := mgr.DeleteRun(ctx, id); err != nil {
						return historyMsg{err: err}
					}
					h, err := mgr.ListRuns(ctx, 30, 0)
					return historyMsg{history: h, err: err}
				},
			}
			return nil
		}
		return nil
	}

	var cmd tea.Cmd
	m.historyList, cmd = m.historyList.Update(msg)
	return cmd
}

// historyDetailContent renders a single run's per-request results.
func (m model) historyDetailContent() string {
	r := m.historyDetail
	if r == nil {
		return m.styles.muted.Render("No run selected.")
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "%s  %s\n", m.styles.accent.Render(fmt.Sprintf("#%d", r.ID)), r.FilePath)
	fmt.Fprintf(&sb, "%s   started %s   %.1fs\n\n", r.Environment, r.StartedAt, float64(r.DurationMs)/1000.0)
	fmt.Fprintf(&sb, "%s passed · %s failed · %s skipped (of %d)\n",
		m.styles.success.Render(fmt.Sprint(r.Passed)),
		m.styles.danger.Render(fmt.Sprint(r.Failed)),
		m.styles.warn.Render(fmt.Sprint(r.Skipped)), r.Total)

	for _, rr := range r.Results {
		state := m.styles.success.Render("✓")
		switch {
		case rr.Skipped:
			state = m.styles.warn.Render("⊘")
		case !rr.Passed:
			state = m.styles.danger.Render("✗")
		}
		fmt.Fprintf(&sb, "\n%s %s   %s %s   %d   %dms\n", state, rr.RequestName, rr.Method, rr.URL, rr.StatusCode, rr.DurationMs)
		if rr.Error != "" {
			sb.WriteString(m.styles.danger.Render("  "+rr.Error) + "\n")
		}
		for _, a := range rr.Assertions {
			glyph := m.styles.success.Render("  ✓")
			if !a.Passed {
				glyph = m.styles.danger.Render("  ✗")
			}
			fmt.Fprintf(&sb, "%s %s %s %s\n", glyph, a.Subject, a.Operator, a.Expected)
		}
		if rr.BodyPreview != "" {
			sb.WriteString(m.styles.muted.Render("  "+truncateText(rr.BodyPreview, 200)) + "\n")
		}
	}
	return sb.String()
}
