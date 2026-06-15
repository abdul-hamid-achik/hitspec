package tui

import (
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/abdul-hamid-achik/hitspec/packages/clientmgr"
)

type responseTab int

const (
	respBody responseTab = iota
	respHeaders
	respAssertions
	respTiming
	respCaptures
)

var responseTabNames = []string{"Body", "Headers", "Assertions", "Timing", "Captures"}

// responseViewer is a reusable tabbed view over a run result: a Body tab with
// pretty-printed, syntax-highlighted payload, plus Headers, Assertions, Timing
// and Captures tabs. It owns a viewport and reserves one row for the tab bar.
type responseViewer struct {
	vp          viewport.Model
	tab         responseTab
	result      *clientmgr.RunResultDTO
	placeholder string
	styles      styles
	color       bool
	width       int // pane width, so the tab bar can collapse when it won't fit
}

func newResponseViewer(s styles, color bool) responseViewer {
	vp := viewport.New(viewport.WithWidth(60), viewport.WithHeight(18))
	vp.MouseWheelEnabled = true
	r := responseViewer{vp: vp, styles: s, color: color, placeholder: "No response yet."}
	r.vp.SetContent(r.placeholder)
	return r
}

func (r *responseViewer) setSize(w, h int) {
	r.width = max(4, w)
	r.vp.SetWidth(max(4, w))
	r.vp.SetHeight(max(1, h-1)) // reserve one row for the tab bar
}

// setResult installs a run result and renders the active tab.
func (r *responseViewer) setResult(result *clientmgr.RunResultDTO) {
	r.result = result
	r.rebuild()
}

// setPlaceholder shows arbitrary text (file summary, exported snippet) and hides
// the tab bar until the next result arrives.
func (r *responseViewer) setPlaceholder(text string) {
	r.result = nil
	r.placeholder = text
	r.vp.SetContent(text)
	r.vp.GotoTop()
}

// setStyles re-themes the viewer in place, preserving the current tab/result.
func (r *responseViewer) setStyles(s styles) {
	r.styles = s
	r.rebuild()
}

func (r *responseViewer) nextTab() {
	r.tab = (r.tab + 1) % responseTab(len(responseTabNames))
	r.rebuild()
}

func (r *responseViewer) prevTab() {
	n := responseTab(len(responseTabNames))
	r.tab = (r.tab + n - 1) % n
	r.rebuild()
}

func (r *responseViewer) update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	r.vp, cmd = r.vp.Update(msg)
	return cmd
}

func (r *responseViewer) rebuild() {
	if r.result == nil {
		r.vp.SetContent(r.placeholder)
		return
	}
	r.vp.SetContent(r.tabContent())
	r.vp.GotoTop()
}

func (r responseViewer) hasResult() bool { return r.result != nil }

func (r responseViewer) view() string {
	if r.result == nil {
		return r.vp.View()
	}
	return lipgloss.JoinVertical(lipgloss.Left, r.tabBar(), r.vp.View())
}

func (r responseViewer) tabBar() string {
	parts := make([]string, 0, len(responseTabNames))
	for i, name := range responseTabNames {
		if responseTab(i) == r.tab {
			parts = append(parts, r.styles.tag.Render(name)) // tag carries its own padding
		} else {
			parts = append(parts, r.styles.muted.Render(name))
		}
	}
	full := strings.Join(parts, " ")
	// When the pane is too narrow for the whole strip, collapse to just the
	// active tab plus a position indicator (←→/[] cycle the tabs) so the label is
	// never sliced mid-word by the panel's clip.
	if r.width > 0 && lipgloss.Width(full) > r.width {
		active := r.styles.tag.Render(responseTabNames[r.tab])
		pos := r.styles.muted.Render(fmt.Sprintf(" %d/%d", int(r.tab)+1, len(responseTabNames)))
		return active + pos
	}
	return full
}

func (r responseViewer) tabContent() string {
	switch r.tab {
	case respBody:
		return r.bodyTab()
	case respHeaders:
		return r.headersTab()
	case respAssertions:
		return r.assertionsTab()
	case respTiming:
		return r.timingTab()
	case respCaptures:
		return r.capturesTab()
	}
	return ""
}

// primary returns the last request result that produced a response, falling back
// to the final result. Body/Headers tabs inspect this one.
func (r responseViewer) primary() *clientmgr.RequestResultDTO {
	for i := len(r.result.Results) - 1; i >= 0; i-- {
		if r.result.Results[i].Response != nil {
			return &r.result.Results[i]
		}
	}
	if len(r.result.Results) > 0 {
		return &r.result.Results[len(r.result.Results)-1]
	}
	return nil
}

func (r responseViewer) bodyTab() string {
	p := r.primary()
	if p == nil {
		return "No requests executed."
	}
	var sb strings.Builder
	if p.Response != nil {
		sb.WriteString(statusLine(r.styles, p.Response) + "\n\n")
		if p.Response.Body != "" {
			body := p.Response.Body
			truncated := false
			if len(body) > maxResponseBodyRender {
				// Back up to a rune boundary so we never hand invalid UTF-8 to
				// the highlighter or the viewport.
				cut := maxResponseBodyRender
				for cut > 0 && !utf8.RuneStart(body[cut]) {
					cut--
				}
				body = body[:cut]
				truncated = true
			}
			ct := headerValue(p.Response.Headers, "Content-Type")
			sb.WriteString(formatBody(body, ct, r.color))
			if truncated {
				sb.WriteString("\n" + r.styles.muted.Render("…[truncated]"))
			}
		} else {
			sb.WriteString(r.styles.muted.Render("(empty body)"))
		}
	} else if p.Error != "" {
		sb.WriteString(r.styles.danger.Render(p.Error))
	} else {
		sb.WriteString(r.styles.muted.Render("No response."))
	}
	return sb.String()
}

func (r responseViewer) headersTab() string {
	p := r.primary()
	if p == nil || p.Response == nil {
		return r.styles.muted.Render("No response headers.")
	}
	keys := sortedStringKeys(p.Response.Headers)
	if len(keys) == 0 {
		return r.styles.muted.Render("No response headers.")
	}
	var sb strings.Builder
	for _, k := range keys {
		sb.WriteString(r.styles.accent.Render(k) + ": " + p.Response.Headers[k] + "\n")
	}
	return sb.String()
}

func (r responseViewer) assertionsTab() string {
	var sb strings.Builder
	total, passed := 0, 0
	for _, rr := range r.result.Results {
		if len(rr.Assertions) == 0 {
			continue
		}
		sb.WriteString(r.styles.title.Render(rr.Name) + "\n")
		for _, a := range rr.Assertions {
			total++
			glyph := r.styles.danger.Render("✗")
			if a.Passed {
				glyph = r.styles.success.Render("✓")
				passed++
			}
			line := fmt.Sprintf("  %s %s %s %v", glyph, a.Subject, a.Operator, a.Expected)
			if !a.Passed && a.Message != "" {
				line += r.styles.muted.Render("  (" + a.Message + ")")
			}
			sb.WriteString(line + "\n")
		}
	}
	if total == 0 {
		return r.styles.muted.Render("No assertions defined.")
	}
	return fmt.Sprintf("%d/%d passed\n\n", passed, total) + sb.String()
}

func (r responseViewer) timingTab() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "Total %.0fms   %s passed · %s failed · %s skipped\n\n",
		r.result.Duration,
		r.styles.success.Render(fmt.Sprint(r.result.Passed)),
		r.styles.danger.Render(fmt.Sprint(r.result.Failed)),
		r.styles.warn.Render(fmt.Sprint(r.result.Skipped)))
	for _, rr := range r.result.Results {
		state := r.styles.success.Render("✓")
		switch {
		case rr.Skipped:
			state = r.styles.warn.Render("⊘")
		case !rr.Passed:
			state = r.styles.danger.Render("✗")
		}
		extra := ""
		if rr.Response != nil {
			extra = fmt.Sprintf("  %d  %s", rr.Response.StatusCode, humanBytes(rr.Response.Size))
		}
		fmt.Fprintf(&sb, "%s %-28s %6.0fms%s\n", state, truncateText(rr.Name, 28), rr.Duration, extra)
	}
	return sb.String()
}

func (r responseViewer) capturesTab() string {
	var sb strings.Builder
	found := false
	for _, rr := range r.result.Results {
		if len(rr.Captures) == 0 {
			continue
		}
		found = true
		sb.WriteString(r.styles.title.Render(rr.Name) + "\n")
		for _, k := range sortedAnyKeys(rr.Captures) {
			fmt.Fprintf(&sb, "  %s = %v\n", r.styles.accent.Render(k), rr.Captures[k])
		}
	}
	if !found {
		return r.styles.muted.Render("No captured variables.")
	}
	return sb.String()
}

// statusLine renders a response status code (colored by class), latency and size.
func statusLine(s styles, resp *clientmgr.HTTPResponseDTO) string {
	text := fmt.Sprintf("%d %s", resp.StatusCode, resp.Status)
	style := s.success
	switch {
	case resp.StatusCode >= 500 || resp.StatusCode == 0:
		style = s.danger
	case resp.StatusCode >= 400:
		style = s.warn
	case resp.StatusCode >= 300:
		style = s.accent
	}
	return fmt.Sprintf("%s  %.0fms  %s", style.Render(text), resp.Duration, humanBytes(resp.Size))
}

func sortedStringKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func sortedAnyKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
