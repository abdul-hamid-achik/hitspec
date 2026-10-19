package tui

import (
	"fmt"
	"sort"
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/abdul-hamid-achik/hitspec/v2/packages/clientmgr"
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
	sel         int // index into result.Results of the request the Body/Headers tabs show
	placeholder string
	styles      styles
	color       bool
	width       int // pane width, so the tab bar can collapse when it won't fit
}

func newResponseViewer(s styles, color bool) responseViewer {
	vp := viewport.New(viewport.WithWidth(60), viewport.WithHeight(18))
	vp.MouseWheelEnabled = true
	vp.SoftWrap = true // long lines and long error messages wrap instead of being cut
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
	r.sel = defaultSelection(result)
	r.rebuild()
}

// defaultSelection picks the request a fresh result opens on: the first one that
// failed (what a run is usually opened to find out), else the last one that got a
// response, else the last one.
func defaultSelection(result *clientmgr.RunResultDTO) int {
	if result == nil || len(result.Results) == 0 {
		return 0
	}
	for i, rr := range result.Results {
		if !rr.Skipped && !rr.Passed {
			return i
		}
	}
	for i := len(result.Results) - 1; i >= 0; i-- {
		if result.Results[i].Response != nil {
			return i
		}
	}
	return len(result.Results) - 1
}

// selectRelative moves the shown request by delta within a multi-request result.
func (r *responseViewer) selectRelative(delta int) {
	if r.result == nil || len(r.result.Results) < 2 {
		return
	}
	r.sel = clamp(r.sel+delta, 0, len(r.result.Results)-1)
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

// multiResult reports whether the shown result holds several requests (a file
// run), so the viewer can switch between them.
func (r responseViewer) multiResult() bool { return r.result != nil && len(r.result.Results) > 1 }

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

// primary returns the request result the Body and Headers tabs inspect.
func (r responseViewer) primary() *clientmgr.RequestResultDTO {
	if r.sel >= 0 && r.sel < len(r.result.Results) {
		return &r.result.Results[r.sel]
	}
	if n := len(r.result.Results); n > 0 {
		return &r.result.Results[n-1]
	}
	return nil
}

// resultTitle names a request result: its @name, else "METHOD URL" of what was
// sent, else its position.
func resultTitle(rr clientmgr.RequestResultDTO, i int) string {
	switch {
	case rr.Name != "":
		return rr.Name
	case rr.Request != nil && rr.Request.URL != "":
		return rr.Request.Method + " " + rr.Request.URL
	}
	return fmt.Sprintf("request %d", i+1)
}

// requestBar tells which request of a multi-request run the tab shows and how to
// move between them.
func (r responseViewer) requestBar() string {
	n := len(r.result.Results)
	if n < 2 {
		return ""
	}
	rr := r.result.Results[r.sel]
	mark := r.styles.success.Render("✓")
	switch {
	case rr.Skipped:
		mark = r.styles.warn.Render("⊘")
	case !rr.Passed:
		mark = r.styles.danger.Render("✗")
	}
	return fmt.Sprintf("%s %s %s\n%s\n\n", r.styles.accent.Render(fmt.Sprintf("request %d/%d", r.sel+1, n)),
		mark, resultTitle(rr, r.sel), r.styles.muted.Render("< > or , . switch request"))
}

func (r responseViewer) bodyTab() string {
	p := r.primary()
	if p == nil {
		return "No requests executed."
	}
	var sb strings.Builder
	sb.WriteString(r.requestBar())
	if p.Response != nil {
		sb.WriteString(statusLine(r.styles, p.Response) + "\n\n")
		if p.Response.Body != "" {
			text, lexer, cut := prepareBody(p.Response.Body, headerValue(p.Response.Headers, "Content-Type"))
			sb.WriteString(highlight(text, lexer, r.color))
			if cut != nil {
				sb.WriteString("\n" + r.styles.muted.Render(cut.String()))
			}
		} else {
			sb.WriteString(r.styles.muted.Render("(empty body)"))
		}
	} else if p.Error != "" {
		sb.WriteString(r.styles.danger.Render(p.Error))
	} else if p.Skipped {
		reason := p.SkipReason
		if reason == "" {
			reason = "skipped"
		}
		sb.WriteString(r.styles.warn.Render("skipped: " + reason))
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
	sb.WriteString(r.requestBar())
	for _, k := range keys {
		sb.WriteString(r.styles.accent.Render(k) + ": " + p.Response.Headers[k] + "\n")
	}
	return sb.String()
}

func (r responseViewer) assertionsTab() string {
	var sb strings.Builder
	total, passed := 0, 0
	for i, rr := range r.result.Results {
		if len(rr.Assertions) == 0 {
			continue
		}
		sb.WriteString(r.styles.title.Render(resultTitle(rr, i)) + "\n")
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
	for i, rr := range r.result.Results {
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
		cursor := ""
		if len(r.result.Results) > 1 { // mark the request the Body/Headers tabs show
			cursor = " "
			if i == r.sel {
				cursor = r.styles.accent.Render("▸")
			}
		}
		fmt.Fprintf(&sb, "%s%s %-28s %6.0fms%s\n", cursor, state, truncateText(resultTitle(rr, i), 28), rr.Duration, extra)
	}
	return sb.String()
}

func (r responseViewer) capturesTab() string {
	var sb strings.Builder
	found := false
	for i, rr := range r.result.Results {
		if len(rr.Captures) == 0 {
			continue
		}
		found = true
		sb.WriteString(r.styles.title.Render(resultTitle(rr, i)) + "\n")
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
