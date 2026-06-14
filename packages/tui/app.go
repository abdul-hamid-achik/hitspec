package tui

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"charm.land/bubbles/v2/filepicker"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/progress"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/table"
	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/abdul-hamid-achik/hitspec/packages/clientmgr"
)

type model struct {
	ctx    context.Context
	mgr    *clientmgr.Manager
	events <-chan clientmgr.Event
	keys   keyMap
	styles styles
	opts   Options

	width  int
	height int
	screen screen
	focus  focusPane

	workspace clientmgr.WorkspaceDTO
	files     []clientmgr.FileInfoDTO
	selected  string
	raw       string
	parsed    *clientmgr.ParsedFileDTO
	dirty     bool
	editing   bool
	loading   bool
	status    string
	err       string
	showHelp  bool
	color     bool // syntax highlighting / ANSI color enabled (disabled in golden tests)

	lastResult   *clientmgr.RunResultDTO
	progress     clientmgr.RequestProgress
	stress       clientmgr.StressStatusDTO
	stressResult *clientmgr.StressResultDTO
	mock         clientmgr.MockStatusDTO
	record       clientmgr.RecordStatusDTO
	history      clientmgr.HistoryListDTO
	cookies      []clientmgr.CookieDTO
	config       clientmgr.ConfigDTO
	envs         []clientmgr.EnvironmentDTO
	contracts    []clientmgr.ContractResultDTO

	filesList   list.Model
	palette     list.Model
	commandOpen bool
	envList     list.Model
	envOpen     bool
	themeList   list.Model
	themeOpen   bool
	theme       string

	searchInput   textinput.Model
	searchList    list.Model
	searchOpen    bool
	searchResults []clientmgr.SearchResultDTO

	historyList       list.Model
	historyDetail     *clientmgr.HistoryRunDTO
	historyDetailMode bool

	requests    table.Model
	headers     table.Model
	assertions  table.Model
	source      textarea.Model
	respView    responseViewer
	preview     viewport.Model
	progressBar progress.Model
	spinner     spinner.Model
	filepicker  filepicker.Model

	formInputs []textinput.Model
	formFocus  int
	formMode   string
	formActive bool

	toasts  toastCenter
	confirm *confirmState
	prompt  *promptState

	// transitioned is set within a single Update tick when a key opened/closed a
	// modal or moved focus into a text widget, so the same key is not also
	// re-delivered to that widget (the Bubble Tea "double-process" pitfall). It
	// is reset at the top of every Update.
	transitioned bool
}

// notify pushes a toast notification and returns the command that expires it.
func (m *model) notify(severity toastSeverity, text string) tea.Cmd {
	var cmd tea.Cmd
	m.toasts, cmd = m.toasts.push(severity, text)
	return cmd
}

type fileItem struct {
	path  string
	name  string
	count int
}

func (i fileItem) FilterValue() string { return i.path }
func (i fileItem) Title() string       { return i.name }
func (i fileItem) Description() string {
	return fmt.Sprintf("%s  %d requests", i.path, i.count)
}

type commandItem struct {
	title string
	desc  string
	id    string
}

func (i commandItem) FilterValue() string { return i.title + " " + i.desc }
func (i commandItem) Title() string       { return i.title }
func (i commandItem) Description() string { return i.desc }

type envItem struct {
	name   string
	vars   int
	active bool
}

func (i envItem) FilterValue() string { return i.name }
func (i envItem) Title() string {
	if i.active {
		return i.name + " ●"
	}
	return i.name
}
func (i envItem) Description() string { return fmt.Sprintf("%d variables", i.vars) }

type filesLoadedMsg struct {
	workspace clientmgr.WorkspaceDTO
	files     []clientmgr.FileInfoDTO
	err       error
}

type fileLoadedMsg struct {
	path   string
	raw    string
	parsed *clientmgr.ParsedFileDTO
	err    error
}

type runDoneMsg struct {
	result *clientmgr.RunResultDTO
	err    error
	adhoc  bool // ad-hoc runs surface the response pane even from another screen
}

type simpleMsg struct {
	kind string
	err  error
}

type historyMsg struct {
	history clientmgr.HistoryListDTO
	err     error
}

type cookiesMsg struct {
	cookies []clientmgr.CookieDTO
	err     error
}

type configMsg struct {
	config    clientmgr.ConfigDTO
	envs      []clientmgr.EnvironmentDTO
	workspace *clientmgr.WorkspaceDTO // set by the settings save so the topbar/form reflect the new active env
	err       error
}

type contractMsg struct {
	results []clientmgr.ContractResultDTO
	err     error
}

type importMsg struct {
	path   string
	raw    string
	parsed *clientmgr.ParsedFileDTO
	err    error
}

// fileRenamedMsg is emitted after a rename/duplicate: it opens the resulting
// file and refreshes the file list. action is "renamed" or "duplicated".
type fileRenamedMsg struct {
	path   string
	raw    string
	parsed *clientmgr.ParsedFileDTO
	action string
	err    error
}

type previewMsg struct {
	title   string
	content string
	err     error
}

// copyMsg is emitted by clipboard/export actions. content, when present, is the
// rendered snippet (e.g. a curl command) surfaced so the user can review what
// was copied; title is the status line.
type copyMsg struct {
	title   string
	content string
	err     error
}

// envSelectedMsg is emitted after the environment switcher activates an
// environment; it carries refreshed config/workspace so the topbar updates.
type envSelectedMsg struct {
	name      string
	config    clientmgr.ConfigDTO
	envs      []clientmgr.EnvironmentDTO
	workspace clientmgr.WorkspaceDTO
	err       error
}

type managerEventMsg clientmgr.Event

func newModel(ctx context.Context, mgr *clientmgr.Manager, opts Options) model {
	p := defaultPalette()
	if found, ok := paletteByName(opts.Theme); ok {
		p = found
	}
	s := newStyles(p)
	km := defaultKeyMap()
	filesList := list.New(nil, list.NewDefaultDelegate(), 30, 20)
	filesList.Title = "Files"
	filesList.DisableQuitKeybindings()
	filesList.SetShowHelp(false)
	filesList.SetShowPagination(false)

	cmdPalette := list.New(buildCommandItems(), list.NewDefaultDelegate(), 48, 16)
	cmdPalette.Title = "Command Palette"
	cmdPalette.DisableQuitKeybindings()

	themeList := list.New(buildThemeItems(p.name), list.NewDefaultDelegate(), 40, 14)
	themeList.Title = "Switch Theme"
	themeList.DisableQuitKeybindings()
	themeList.SetShowHelp(false)

	envList := list.New(nil, list.NewDefaultDelegate(), 40, 14)
	envList.Title = "Switch Environment"
	envList.DisableQuitKeybindings()
	envList.SetShowHelp(false)

	searchInput := textinput.New()
	searchInput.Placeholder = "search requests by name, method, URL, or tag…"
	searchInput.CharLimit = 256
	searchInput.SetWidth(56)

	searchList := list.New(nil, list.NewDefaultDelegate(), 60, 14)
	searchList.Title = "Search Requests"
	searchList.DisableQuitKeybindings()
	searchList.SetShowHelp(false)

	historyList := list.New(nil, list.NewDefaultDelegate(), 60, 20)
	historyList.Title = "Run History"
	historyList.DisableQuitKeybindings()
	historyList.SetShowHelp(false)

	requests := table.New(
		table.WithColumns([]table.Column{
			{Title: "Name", Width: 22},
			{Title: "Method", Width: 8},
			{Title: "URL", Width: 42},
		}),
		table.WithFocused(true),
	)
	headers := table.New(table.WithColumns([]table.Column{{Title: "Header", Width: 18}, {Title: "Value", Width: 42}}))
	assertions := table.New(table.WithColumns([]table.Column{{Title: "Subject", Width: 18}, {Title: "Op", Width: 12}, {Title: "Expected", Width: 28}}))

	source := textarea.New()
	source.Placeholder = "Open a .http file"
	source.Prompt = ""
	source.ShowLineNumbers = true
	source.Blur()

	respView := newResponseViewer(s, true)
	preview := viewport.New(viewport.WithWidth(60), viewport.WithHeight(20))
	preview.MouseWheelEnabled = true

	bar := progress.New(progress.WithDefaultBlend())
	spin := spinner.New()
	spin.Spinner = spinner.Line
	spin.Style = s.accent
	fp := filepicker.New()

	return model{
		ctx:         ctx,
		mgr:         mgr,
		events:      mgr.Subscribe(ctx),
		keys:        km,
		styles:      s,
		opts:        opts,
		screen:      screenWorkspace,
		focus:       focusFiles,
		status:      "ready",
		color:       true,
		theme:       p.name,
		toasts:      newToastCenter(),
		filesList:   filesList,
		palette:     cmdPalette,
		themeList:   themeList,
		envList:     envList,
		searchInput: searchInput,
		searchList:  searchList,
		historyList: historyList,
		requests:    requests,
		headers:     headers,
		assertions:  assertions,
		source:      source,
		respView:    respView,
		preview:     preview,
		progressBar: bar,
		spinner:     spin,
		filepicker:  fp,
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(loadFilesCmd(m.ctx, m.mgr), waitEventCmd(m.events), m.spinner.Tick)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	m.transitioned = false
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resize()
	case tea.KeyPressMsg:
		cmd := m.handleKey(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	case filesLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err.Error()
			cmds = append(cmds, m.notify(toastError, msg.err.Error()))
			break
		}
		m.err = ""
		m.workspace = msg.workspace
		m.files = msg.files
		m.refreshFileList()
		m.status = fmt.Sprintf("%d files, %d requests", len(msg.files), msg.workspace.TotalRequests)
		// If the selected file no longer exists (e.g. it was just deleted), drop
		// the stale selection so the topbar/source don't reference a gone file.
		if m.selected != "" && !containsFile(msg.files, m.selected) {
			m.selected = ""
			m.parsed = nil
			m.raw = ""
			m.source.SetValue("")
			m.respView.setPlaceholder(m.fileSummary())
		}
		if m.selected == "" && len(msg.files) > 0 {
			m.selected = msg.files[0].RelativePath
			cmds = append(cmds, loadFileCmd(m.ctx, m.mgr, m.selected))
		}
	case fileLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err.Error()
			cmds = append(cmds, m.notify(toastError, msg.err.Error()))
			break
		}
		m.err = ""
		m.selected = msg.path
		m.raw = msg.raw
		m.parsed = msg.parsed
		m.source.SetValue(msg.raw)
		m.dirty = false
		m.editing = false
		m.source.Blur()
		m.refreshRequestTables()
		m.respView.setPlaceholder(m.fileSummary())
		m.status = "opened " + msg.path
	case runDoneMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err.Error()
			cmds = append(cmds, m.notify(toastError, msg.err.Error()))
			break
		}
		m.err = ""
		m.lastResult = msg.result
		m.progress = clientmgr.RequestProgress{}
		m.refreshResultViews()
		// Ad-hoc runs can be launched from another screen, so surface the response
		// pane — but never steal focus from an active source edit.
		if msg.adhoc && !(m.editing && m.focus == focusSource) {
			m.screen = screenWorkspace
			m.focus = focusResponse
			m.syncFocus()
		}
		cmds = append(cmds, m.progressBar.SetPercent(1))
		m.status = fmt.Sprintf("run complete: %d passed, %d failed, %d skipped", msg.result.Passed, msg.result.Failed, msg.result.Skipped)
		sev := toastSuccess
		if msg.result.Failed > 0 {
			sev = toastError
		}
		cmds = append(cmds, m.notify(sev, fmt.Sprintf("%d passed · %d failed · %d skipped", msg.result.Passed, msg.result.Failed, msg.result.Skipped)))
	case simpleMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err.Error()
			cmds = append(cmds, m.notify(toastError, msg.err.Error()))
		} else {
			m.err = ""
			m.status = msg.kind
			m.loadScreenState()
			if m.screen != screenWorkspace {
				m.preview.SetContent(m.secondaryContent())
			}
			cmds = append(cmds, m.notify(toastSuccess, msg.kind), loadFilesCmd(m.ctx, m.mgr))
		}
	case historyMsg:
		if msg.err != nil {
			m.err = msg.err.Error()
			cmds = append(cmds, m.notify(toastError, msg.err.Error()))
		} else {
			m.err = ""
			m.history = msg.history
			m.historyDetailMode = false
			m.refreshHistoryList()
			m.preview.SetContent(m.historyContent())
		}
	case runDetailMsg:
		if msg.err != nil {
			m.err = msg.err.Error()
			cmds = append(cmds, m.notify(toastError, msg.err.Error()))
		} else {
			m.err = ""
			run := msg.run
			m.historyDetail = &run
			m.historyDetailMode = true
			m.preview.SetContent(m.historyDetailContent())
		}
	case searchResultMsg:
		if msg.err != nil {
			m.err = msg.err.Error()
			cmds = append(cmds, m.notify(toastError, msg.err.Error()))
		} else if msg.query == m.searchInput.Value() { // ignore stale responses
			m.err = ""
			m.searchResults = msg.results
			m.refreshSearchList()
		}
	case cookiesMsg:
		if msg.err != nil {
			m.err = msg.err.Error()
			cmds = append(cmds, m.notify(toastError, msg.err.Error()))
		} else {
			m.err = ""
			m.cookies = msg.cookies
			m.preview.SetContent(m.cookiesContent())
		}
	case configMsg:
		if msg.err != nil {
			m.err = msg.err.Error()
			cmds = append(cmds, m.notify(toastError, msg.err.Error()))
		} else {
			m.err = ""
			m.config = msg.config
			m.envs = msg.envs
			if msg.workspace != nil { // settings save: reflect the new active env
				m.workspace = *msg.workspace
			}
			m.refreshEnvList()
			if m.screen == screenSettings {
				m.initFormForScreen(screenSettings)
			}
			m.preview.SetContent(m.settingsContent())
		}
	case envSelectedMsg:
		if msg.err != nil {
			m.err = msg.err.Error()
			cmds = append(cmds, m.notify(toastError, msg.err.Error()))
			break
		}
		m.err = ""
		m.config = msg.config
		m.envs = msg.envs
		m.workspace = msg.workspace
		m.refreshEnvList()
		if m.screen == screenSettings {
			m.preview.SetContent(m.settingsContent())
		}
		m.status = "environment: " + msg.name
		cmds = append(cmds, m.notify(toastSuccess, "environment → "+msg.name), loadFilesCmd(m.ctx, m.mgr))
	case contractMsg:
		if msg.err != nil {
			m.err = msg.err.Error()
			cmds = append(cmds, m.notify(toastError, msg.err.Error()))
		} else {
			m.err = ""
			m.contracts = msg.results
			m.preview.SetContent(m.contractContent())
			m.status = fmt.Sprintf("contract verification complete: %d files", len(msg.results))
			cmds = append(cmds, m.notify(toastInfo, m.status))
		}
	case importMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err.Error()
			cmds = append(cmds, m.notify(toastError, msg.err.Error()))
		} else {
			m.err = ""
			m.screen = screenWorkspace
			m.selected = msg.path
			m.raw = msg.raw
			m.parsed = msg.parsed
			m.source.SetValue(msg.raw)
			m.refreshRequestTables()
			m.respView.setPlaceholder(m.fileSummary())
			m.status = "imported " + msg.path
			cmds = append(cmds, m.notify(toastSuccess, "imported "+msg.path), loadFilesCmd(m.ctx, m.mgr))
		}
	case fileRenamedMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err.Error()
			cmds = append(cmds, m.notify(toastError, msg.err.Error()))
		} else {
			m.err = ""
			m.screen = screenWorkspace
			m.selected = msg.path
			m.raw = msg.raw
			m.parsed = msg.parsed
			m.source.SetValue(msg.raw)
			m.dirty = false
			m.editing = false
			m.source.Blur()
			m.refreshRequestTables()
			m.respView.setPlaceholder(m.fileSummary())
			m.status = msg.action + " → " + msg.path
			cmds = append(cmds, m.notify(toastSuccess, msg.action+" → "+msg.path), loadFilesCmd(m.ctx, m.mgr))
		}
	case previewMsg:
		if msg.err != nil {
			m.err = msg.err.Error()
			cmds = append(cmds, m.notify(toastError, msg.err.Error()))
		} else {
			m.err = ""
			m.preview.SetContent(msg.content)
			m.status = msg.title
		}
	case copyMsg:
		if msg.err != nil {
			m.err = msg.err.Error()
			cmds = append(cmds, m.notify(toastError, msg.err.Error()))
		} else {
			m.err = ""
			m.status = msg.title
			cmds = append(cmds, m.notify(toastSuccess, msg.title))
			if msg.content != "" {
				if m.screen == screenWorkspace {
					m.respView.setPlaceholder(msg.content)
				} else {
					m.preview.SetContent(msg.content)
				}
			}
		}
	case managerEventMsg:
		ev := clientmgr.Event(msg)
		m.handleEvent(ev)
		cmds = append(cmds, waitEventCmd(m.events))
	case toastExpiredMsg:
		m.toasts = m.toasts.expire(msg.id)
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)
	case progress.FrameMsg:
		var cmd tea.Cmd
		m.progressBar, cmd = m.progressBar.Update(msg)
		cmds = append(cmds, cmd)
	}

	// A modal dialog captures all input, and a key that just opened/closed an
	// overlay or moved focus into a widget must not also be re-delivered to a
	// background widget (the Bubble Tea double-process pitfall). This runs before
	// the overlay-forwarding blocks so an open overlay still receives navigation
	// keys, but the key that closes it does not leak through.
	if m.confirm != nil || m.transitioned {
		return m, tea.Batch(cmds...)
	}

	if m.prompt != nil {
		var cmd tea.Cmd
		m.prompt.input, cmd = m.prompt.input.Update(msg)
		cmds = append(cmds, cmd)
		return m, tea.Batch(cmds...)
	}

	if m.searchOpen {
		if kp, ok := msg.(tea.KeyPressMsg); ok {
			switch kp.String() {
			case "up", "down", "ctrl+p", "ctrl+n", "pgup", "pgdown":
				var cmd tea.Cmd
				m.searchList, cmd = m.searchList.Update(msg)
				cmds = append(cmds, cmd)
				return m, tea.Batch(cmds...)
			}
		}
		old := m.searchInput.Value()
		var cmd tea.Cmd
		m.searchInput, cmd = m.searchInput.Update(msg)
		cmds = append(cmds, cmd)
		if m.searchInput.Value() != old {
			cmds = append(cmds, searchRequestsCmd(m.ctx, m.mgr, m.searchInput.Value()))
		}
		return m, tea.Batch(cmds...)
	}

	if m.envOpen {
		var cmd tea.Cmd
		m.envList, cmd = m.envList.Update(msg)
		cmds = append(cmds, cmd)
		return m, tea.Batch(cmds...)
	}

	if m.themeOpen {
		var cmd tea.Cmd
		m.themeList, cmd = m.themeList.Update(msg)
		cmds = append(cmds, cmd)
		return m, tea.Batch(cmds...)
	}

	if m.commandOpen {
		var cmd tea.Cmd
		m.palette, cmd = m.palette.Update(msg)
		cmds = append(cmds, cmd)
		return m, tea.Batch(cmds...)
	}

	if m.formActive && m.screen != screenWorkspace && len(m.formInputs) > 0 {
		var cmd tea.Cmd
		m.formInputs[m.formFocus], cmd = m.formInputs[m.formFocus].Update(msg)
		cmds = append(cmds, cmd)
		// Rebuild the rendered form only on real input, not on spinner/progress ticks.
		if _, ok := msg.(tea.KeyPressMsg); ok {
			m.preview.SetContent(m.secondaryContent())
		}
	}

	if m.editing && m.focus == focusSource {
		var cmd tea.Cmd
		old := m.source.Value()
		m.source, cmd = m.source.Update(msg)
		if m.source.Value() != old {
			m.dirty = true
		}
		cmds = append(cmds, cmd)
	}
	if m.focus == focusResponse {
		if kp, ok := msg.(tea.KeyPressMsg); ok && m.respView.hasResult() {
			switch kp.String() {
			case "right", "]":
				m.respView.nextTab()
				return m, tea.Batch(cmds...)
			case "left", "[":
				m.respView.prevTab()
				return m, tea.Batch(cmds...)
			}
		}
		cmds = append(cmds, m.respView.update(msg))
	}
	if m.screen != screenWorkspace {
		var cmd tea.Cmd
		m.preview, cmd = m.preview.Update(msg)
		cmds = append(cmds, cmd)
	}
	return m, tea.Batch(cmds...)
}

func (m model) View() tea.View {
	content := m.render()
	var v tea.View
	v.SetContent(content)
	v.AltScreen = true
	if m.opts.Mouse {
		v.MouseMode = tea.MouseModeCellMotion
	}
	return v
}

func (m *model) handleKey(msg tea.KeyPressMsg) tea.Cmd {
	// m.err is intentionally NOT cleared here: the status bar is the only
	// persistent view of an error, so wiping it on every keypress (including ?
	// for help or pane navigation) hid errors instantly. It is cleared instead
	// when a new operation or navigation actually begins (setScreen, the run/
	// save/load command builders) and replaced on the next success.
	// The help overlay is modal: any key dismisses it (open with ?, close with
	// anything). Captured here so the key doesn't also act on the screen behind.
	if m.showHelp {
		m.transitioned = true
		m.showHelp = false
		return nil
	}
	if m.confirm != nil {
		m.transitioned = true
		switch msg.String() {
		case "y", "Y", "enter":
			action := m.confirm.action
			m.confirm = nil
			return action
		case "n", "N", "esc":
			m.confirm = nil
			return nil
		}
		return nil
	}
	if m.prompt != nil {
		switch {
		case key.Matches(msg, m.keys.Confirm):
			m.transitioned = true
			value := m.prompt.input.Value()
			onSubmit := m.prompt.onSubmit
			m.prompt = nil
			if onSubmit != nil {
				return onSubmit(value)
			}
			return nil
		case key.Matches(msg, m.keys.Cancel):
			m.transitioned = true
			m.prompt = nil
			return nil
		}
		return nil // typing falls through to the prompt-input forwarder in Update
	}
	if m.searchOpen {
		switch {
		case key.Matches(msg, m.keys.Cancel):
			m.transitioned = true
			m.searchOpen = false
			m.searchInput.Blur()
			return nil
		case key.Matches(msg, m.keys.Confirm):
			m.transitioned = true
			m.searchOpen = false
			m.searchInput.Blur()
			if it, ok := m.searchList.SelectedItem().(searchItem); ok {
				m.loading = true
				return loadFileCmd(m.ctx, m.mgr, it.file)
			}
			return nil
		}
		return nil // typing/nav falls through to the search forwarder in Update
	}
	if m.envOpen {
		switch {
		case key.Matches(msg, m.keys.Cancel):
			m.transitioned = true
			m.envOpen = false
			return nil
		case key.Matches(msg, m.keys.Confirm):
			m.transitioned = true
			m.envOpen = false
			if item, ok := m.envList.SelectedItem().(envItem); ok {
				return selectEnvCmd(m.ctx, m.mgr, item.name)
			}
			return nil
		}
		return nil
	}
	if m.themeOpen {
		switch {
		case key.Matches(msg, m.keys.Cancel):
			m.transitioned = true
			m.themeOpen = false
			return nil
		case key.Matches(msg, m.keys.Confirm):
			m.transitioned = true
			m.themeOpen = false
			if item, ok := m.themeList.SelectedItem().(themeItem); ok {
				m.applyTheme(item.name)
				return m.notify(toastSuccess, "theme: "+item.name)
			}
			return nil
		}
		return nil
	}
	if m.commandOpen {
		switch {
		case key.Matches(msg, m.keys.Cancel):
			m.transitioned = true
			m.commandOpen = false
			return nil
		case key.Matches(msg, m.keys.Confirm):
			item, ok := m.palette.SelectedItem().(commandItem)
			if ok {
				m.transitioned = true
				m.commandOpen = false
				return m.executeCommand(item.id)
			}
		}
		return nil
	}
	if m.editing && m.focus == focusSource {
		switch {
		case key.Matches(msg, m.keys.Save):
			return m.saveCmd()
		case key.Matches(msg, m.keys.Cancel):
			m.editing = false
			m.source.Blur()
			if !m.dirty {
				m.source.SetValue(m.raw)
			}
			return nil
		}
		return nil
	}
	switch {
	case key.Matches(msg, m.keys.Quit):
		// While editing source, this case is unreachable — the editing block
		// above forwards every non-save/cancel key (including q) to the textarea,
		// so q is typed into the file rather than quitting.
		//
		// ctrl+c is a hard interrupt — always exit immediately.
		if msg.String() == "ctrl+c" {
			return func() tea.Msg { return tea.Quit() }
		}
		// Unsaved changes: confirm before discarding them.
		if m.dirty {
			m.transitioned = true
			m.confirm = &confirmState{
				title:  "Discard unsaved changes?",
				body:   "Your edits to " + m.selected + " will be lost.",
				action: func() tea.Msg { return tea.Quit() },
			}
			return nil
		}
		return func() tea.Msg { return tea.Quit() }
	case key.Matches(msg, m.keys.Help):
		m.transitioned = true
		m.showHelp = true
	case key.Matches(msg, m.keys.Palette):
		m.transitioned = true
		m.commandOpen = true
	case key.Matches(msg, m.keys.EnvSwitch):
		m.transitioned = true
		m.envOpen = true
		m.refreshEnvList()
		return loadConfigCmd(m.ctx, m.mgr)
	case key.Matches(msg, m.keys.ThemeSwitch):
		m.transitioned = true
		m.themeOpen = true
		m.themeList.SetItems(buildThemeItems(m.theme))
	case key.Matches(msg, m.keys.Search):
		m.transitioned = true
		m.searchOpen = true
		m.searchInput.SetValue("")
		m.searchResults = nil
		m.refreshSearchList()
		return m.searchInput.Focus()
	case key.Matches(msg, m.keys.Workspace):
		m.setScreen(screenWorkspace)
	case key.Matches(msg, m.keys.Stress):
		m.setScreen(screenStress)
	case key.Matches(msg, m.keys.Mock):
		m.setScreen(screenMock)
	case key.Matches(msg, m.keys.Contract):
		m.setScreen(screenContract)
	case key.Matches(msg, m.keys.Record):
		m.setScreen(screenRecord)
	case key.Matches(msg, m.keys.History):
		m.setScreen(screenHistory)
		return loadHistoryCmd(m.ctx, m.mgr)
	case key.Matches(msg, m.keys.Import):
		m.setScreen(screenImport)
	case key.Matches(msg, m.keys.Cookies):
		m.setScreen(screenCookies)
		return loadCookiesCmd(m.ctx, m.mgr)
	case key.Matches(msg, m.keys.Settings):
		m.setScreen(screenSettings)
		return loadConfigCmd(m.ctx, m.mgr)
	}

	if m.screen != screenWorkspace {
		return m.handleSecondaryKey(msg)
	}

	switch {
	case key.Matches(msg, m.keys.Tab):
		m.nextFocus()
	case key.Matches(msg, m.keys.BackTab):
		m.prevFocus()
	case key.Matches(msg, m.keys.Refresh):
		m.loading = true
		return loadFilesCmd(m.ctx, m.mgr)
	case key.Matches(msg, m.keys.Open):
		if m.screen == screenWorkspace && m.focus == focusFiles {
			if item, ok := m.filesList.SelectedItem().(fileItem); ok {
				m.loading = true
				return loadFileCmd(m.ctx, m.mgr, item.path)
			}
		}
	case key.Matches(msg, m.keys.Edit):
		if m.screen == screenWorkspace && m.selected != "" {
			m.focus = focusSource
			m.editing = true
			m.transitioned = true
			return m.source.Focus()
		}
	case key.Matches(msg, m.keys.Save):
		return m.saveCmd()
	case key.Matches(msg, m.keys.RunRequest):
		return m.runRequestCmd()
	case key.Matches(msg, m.keys.RunFile):
		return m.runFileCmd()
	case key.Matches(msg, m.keys.NewFile):
		return m.quickCreateCmd()
	case key.Matches(msg, m.keys.GenerateSample):
		if len(m.files) == 0 {
			m.loading = true
			return scaffoldSampleCmd(m.ctx, m.mgr)
		}
	case key.Matches(msg, m.keys.DeleteFile):
		return m.deleteCmd()
	default:
		if m.screen == screenWorkspace && m.focus == focusFiles {
			var cmd tea.Cmd
			m.filesList, cmd = m.filesList.Update(msg)
			return cmd
		}
		if m.screen == screenWorkspace && m.focus == focusRequests {
			var cmd tea.Cmd
			m.requests, cmd = m.requests.Update(msg)
			return cmd
		}
	}
	return nil
}

func (m *model) handleSecondaryKey(msg tea.KeyPressMsg) tea.Cmd {
	if m.screen == screenHistory {
		return m.handleHistoryKey(msg)
	}
	if m.formActive {
		switch {
		case key.Matches(msg, m.keys.Cancel):
			m.transitioned = true
			m.focusForm(false)
			m.preview.SetContent(m.secondaryContent())
			return nil
		case key.Matches(msg, m.keys.Confirm):
			m.transitioned = true
			m.focusForm(false)
			return m.submitFormCmd()
		case key.Matches(msg, m.keys.FormNext):
			m.transitioned = true
			m.moveFormFocus(1)
			m.preview.SetContent(m.secondaryContent())
			return nil
		case key.Matches(msg, m.keys.FormPrev):
			m.transitioned = true
			m.moveFormFocus(-1)
			m.preview.SetContent(m.secondaryContent())
			return nil
		}
		return nil
	}

	switch {
	case key.Matches(msg, m.keys.Edit):
		m.transitioned = true
		return m.focusForm(true)
	case key.Matches(msg, m.keys.Open), key.Matches(msg, m.keys.Confirm):
		return m.submitFormCmd()
	case key.Matches(msg, m.keys.Refresh):
		return m.refreshScreenCmd()
	}

	switch msg.String() {
	case "s":
		return m.submitFormCmd()
	case "x":
		return m.stopScreenCmd()
	case "E":
		if m.screen == screenRecord {
			return exportRecordCmd(m.ctx, m.mgr)
		}
	case "D":
		if m.screen == screenCookies {
			return m.deleteCookieCmd()
		}
	}
	return nil
}

func (m *model) executeCommand(id string) tea.Cmd {
	switch id {
	case "run-request":
		return m.runRequestCmd()
	case "run-file":
		return m.runFileCmd()
	case "save":
		return m.saveCmd()
	case "edit":
		m.focus = focusSource
		m.editing = true
		m.transitioned = true
		return m.source.Focus()
	case "new-file":
		return m.quickCreateCmd()
	case "scaffold":
		m.loading = true
		return scaffoldSampleCmd(m.ctx, m.mgr)
	case "delete-file":
		return m.deleteCmd()
	case "rename-file":
		if m.selected == "" {
			m.err = "no file selected"
			return nil
		}
		ctx, mgr, old := m.ctx, m.mgr, m.selected
		m.prompt = newPrompt("Rename file", "new/path.http", old, func(newPath string) tea.Cmd {
			return renameFileCmd(ctx, mgr, old, newPath, "renamed")
		})
		return nil
	case "duplicate-file":
		if m.selected == "" {
			m.err = "no file selected"
			return nil
		}
		ctx, mgr, src := m.ctx, m.mgr, m.selected
		m.prompt = newPrompt("Duplicate file", "new/path.http", suggestCopyName(src), func(dst string) tea.Cmd {
			return copyFileCmd(ctx, mgr, src, dst, "duplicated")
		})
		return nil
	case "refresh":
		return loadFilesCmd(m.ctx, m.mgr)
	case "copy-curl":
		return exportCmd(m.ctx, m.mgr, m.selected, m.currentRequestName(), "curl")
	case "copy-httpie":
		return exportCmd(m.ctx, m.mgr, m.selected, m.currentRequestName(), "httpie")
	case "copy-python":
		return exportCmd(m.ctx, m.mgr, m.selected, m.currentRequestName(), "python")
	case "copy-fetch":
		return exportCmd(m.ctx, m.mgr, m.selected, m.currentRequestName(), "fetch")
	case "copy-go":
		return exportCmd(m.ctx, m.mgr, m.selected, m.currentRequestName(), "go")
	case "copy-response":
		return copyTextCmd(m.lastResponseBody(), "response body")
	case "env-switch":
		m.envOpen = true
		m.refreshEnvList()
		return loadConfigCmd(m.ctx, m.mgr)
	case "theme":
		m.themeOpen = true
		m.themeList.SetItems(buildThemeItems(m.theme))
		return nil
	case "search":
		m.searchOpen = true
		m.searchInput.SetValue("")
		m.searchResults = nil
		m.refreshSearchList()
		return m.searchInput.Focus()
	case "adhoc":
		ctx, mgr := m.ctx, m.mgr
		m.prompt = newPrompt("Quick request", "GET https://api.example.com/health", "", func(line string) tea.Cmd {
			return adhocCmd(ctx, mgr, line)
		})
		return nil
	case "stress-start":
		return startStressCmd(m.ctx, m.mgr, m.selected)
	case "stress-stop":
		return func() tea.Msg {
			return simpleMsg{kind: "stress stopping", err: m.mgr.StopStress(m.ctx)}
		}
	case "mock-start":
		return startMockCmd(m.ctx, m.mgr, m.selected)
	case "mock-stop":
		return stopMockCmd(m.ctx, m.mgr)
	case "record-export":
		return exportRecordCmd(m.ctx, m.mgr)
	case "record-clear":
		ctx, mgr := m.ctx, m.mgr
		m.confirm = &confirmState{
			title: "Clear all recordings?",
			body:  "Discards every captured proxy request.",
			action: func() tea.Msg {
				return simpleMsg{kind: "recordings cleared", err: mgr.ClearRecordings(ctx)}
			},
		}
		return nil
	case "history-clear":
		ctx, mgr := m.ctx, m.mgr
		m.confirm = &confirmState{
			title: "Delete all run history?",
			body:  "Permanently removes every persistent run from the database.",
			action: func() tea.Msg {
				if err := mgr.ClearRuns(ctx); err != nil {
					return historyMsg{err: err}
				}
				h, err := mgr.ListRuns(ctx, 30, 0)
				return historyMsg{history: h, err: err}
			},
		}
		return nil
	case "history":
		m.setScreen(screenHistory)
		return loadHistoryCmd(m.ctx, m.mgr)
	case "cookies":
		m.setScreen(screenCookies)
		return loadCookiesCmd(m.ctx, m.mgr)
	case "settings":
		m.setScreen(screenSettings)
		return loadConfigCmd(m.ctx, m.mgr)
	default:
		for i, name := range screenNames {
			if id == name {
				m.setScreen(screen(i))
				return nil
			}
		}
	}
	return nil
}

func (m *model) saveCmd() tea.Cmd {
	if m.selected == "" {
		m.err = "no file selected"
		return nil
	}
	content := m.source.Value()
	m.loading = true
	return func() tea.Msg {
		parsed, err := m.mgr.SaveFile(m.ctx, m.selected, content)
		if err != nil {
			return fileLoadedMsg{path: m.selected, err: err}
		}
		return fileLoadedMsg{path: m.selected, raw: content, parsed: parsed}
	}
}

// currentRequestName returns the name of the request highlighted in the request
// table, or "" which means "every request in the file".
func (m model) currentRequestName() string {
	if row := m.requests.SelectedRow(); len(row) > 0 {
		return row[0]
	}
	return ""
}

// lastResponseBody returns the body of the most recent response, searching from
// the last request backwards so it survives multi-request runs.
func (m model) lastResponseBody() string {
	if m.lastResult == nil {
		return ""
	}
	for i := len(m.lastResult.Results) - 1; i >= 0; i-- {
		if r := m.lastResult.Results[i].Response; r != nil && r.Body != "" {
			return r.Body
		}
	}
	return ""
}

func (m *model) runRequestCmd() tea.Cmd {
	if m.selected == "" {
		m.err = "no file selected"
		return nil
	}
	requestName := m.currentRequestName()
	m.loading = true
	return tea.Batch(m.progressBar.SetPercent(0), func() tea.Msg {
		result, err := m.mgr.Execute(m.ctx, clientmgr.ExecuteReq{File: m.selected, RequestName: requestName})
		return runDoneMsg{result: result, err: err}
	})
}

func (m *model) runFileCmd() tea.Cmd {
	if m.selected == "" {
		m.err = "no file selected"
		return nil
	}
	m.loading = true
	return tea.Batch(m.progressBar.SetPercent(0), func() tea.Msg {
		result, err := m.mgr.RunFile(m.ctx, clientmgr.RunReq{File: m.selected})
		return runDoneMsg{result: result, err: err}
	})
}

func (m *model) quickCreateCmd() tea.Cmd {
	name := fmt.Sprintf("scratch-%d.http", time.Now().Unix())
	m.loading = true
	return func() tea.Msg {
		parsed, err := m.mgr.CreateFile(m.ctx, name, "### New Request\nGET https://example.com\n")
		if err != nil {
			return fileLoadedMsg{path: name, err: err}
		}
		raw, err := m.mgr.ReadFile(m.ctx, name)
		if err != nil {
			return fileLoadedMsg{path: name, err: err}
		}
		return fileLoadedMsg{path: name, raw: raw, parsed: parsed}
	}
}

func (m *model) deleteCmd() tea.Cmd {
	if m.selected == "" {
		m.err = "no file selected"
		return nil
	}
	path := m.selected
	mgr, ctx := m.mgr, m.ctx
	m.confirm = &confirmState{
		title: "Delete file?",
		body:  path,
		action: func() tea.Msg {
			err := mgr.DeleteFile(ctx, path)
			return simpleMsg{kind: "deleted " + path, err: err}
		},
	}
	return nil
}

func (m *model) submitFormCmd() tea.Cmd {
	values := m.formValues()
	switch m.screen {
	case screenStress:
		files := selectedFiles(m.selected)
		rate, _ := strconv.ParseFloat(values["rate"], 64)
		return func() tea.Msg {
			err := m.mgr.StartStress(m.ctx, clientmgr.StressStartReq{
				Files:    files,
				Duration: defaultString(values["duration"], "30s"),
				Rate:     rate,
				VUs:      atoi(values["vus"], 0),
				MaxVUs:   atoi(values["max vus"], 0),
			})
			return simpleMsg{kind: "stress started", err: err}
		}
	case screenMock:
		files := selectedFiles(m.selected)
		return func() tea.Msg {
			_, err := m.mgr.StartMock(m.ctx, clientmgr.MockStartReq{
				Files: files,
				Port:  atoi(values["port"], 3000),
				Delay: values["delay"],
			})
			return simpleMsg{kind: "mock started", err: err}
		}
	case screenContract:
		files := selectedFiles(m.selected)
		return func() tea.Msg {
			results, err := m.mgr.VerifyContracts(m.ctx, clientmgr.ContractVerifyReq{
				Files:        files,
				ProviderURL:  values["provider URL"],
				StateHandler: values["state handler"],
			})
			return contractMsg{results: results, err: err}
		}
	case screenRecord:
		return func() tea.Msg {
			err := m.mgr.StartRecord(m.ctx, clientmgr.RecordStartReq{
				TargetURL:   values["target URL"],
				Port:        atoi(values["port"], 8081),
				Deduplicate: strings.EqualFold(values["deduplicate"], "true"),
			})
			return simpleMsg{kind: "recording proxy started", err: err}
		}
	case screenImport:
		m.loading = true
		return importCmd(m.ctx, m.mgr, values["format"], values["input"], values["base URL"])
	case screenCookies:
		return func() tea.Msg {
			cookies, err := m.mgr.PutCookie(m.ctx, clientmgr.CookieDTO{
				Domain: values["domain"],
				Path:   defaultString(values["path"], "/"),
				Name:   values["name"],
				Value:  values["value"],
			})
			return cookiesMsg{cookies: cookies, err: err}
		}
	case screenSettings:
		ctx, mgr := m.ctx, m.mgr
		return func() tea.Msg {
			cfg := clientmgr.ConfigDTO{
				DefaultEnvironment: values["default env"],
				Timeout:            atoi(values["timeout ms"], 0),
				Retries:            atoi(values["retries"], 0),
				Concurrency:        atoi(values["concurrency"], 0),
			}
			if _, err := mgr.PutConfig(ctx, cfg); err != nil {
				return configMsg{err: err}
			}
			if cfg.DefaultEnvironment != "" {
				_ = mgr.SelectEnvironment(ctx, cfg.DefaultEnvironment)
			}
			// Optionally set/update a single environment variable (read-modify-write).
			if key := strings.TrimSpace(values["set var key"]); key != "" {
				envName := defaultString(values["set var env"], cfg.DefaultEnvironment)
				if err := setEnvVar(ctx, mgr, envName, key, values["set var value"]); err != nil {
					return configMsg{err: err}
				}
			}
			nextCfg, err := mgr.GetConfig(ctx)
			if err != nil {
				return configMsg{err: err}
			}
			envs, err := mgr.ListEnvironments(ctx)
			if err != nil {
				return configMsg{err: err}
			}
			ws, _ := mgr.Workspace(ctx) // refresh so topbar/form show the new active env
			return configMsg{config: nextCfg, envs: envs, workspace: &ws}
		}
	}
	return nil
}

func (m *model) refreshScreenCmd() tea.Cmd {
	switch m.screen {
	case screenHistory:
		return loadHistoryCmd(m.ctx, m.mgr)
	case screenCookies:
		return loadCookiesCmd(m.ctx, m.mgr)
	case screenSettings:
		return loadConfigCmd(m.ctx, m.mgr)
	case screenStress, screenMock, screenRecord:
		m.loadScreenState()
		m.preview.SetContent(m.secondaryContent())
	}
	return nil
}

func (m *model) stopScreenCmd() tea.Cmd {
	switch m.screen {
	case screenStress:
		return func() tea.Msg {
			return simpleMsg{kind: "stress stopping", err: m.mgr.StopStress(m.ctx)}
		}
	case screenMock:
		return stopMockCmd(m.ctx, m.mgr)
	case screenRecord:
		return func() tea.Msg {
			return simpleMsg{kind: "recording proxy stopping", err: m.mgr.StopRecord(m.ctx)}
		}
	}
	return nil
}

func (m *model) deleteCookieCmd() tea.Cmd {
	values := m.formValues()
	domain := values["domain"]
	path := defaultString(values["path"], "/")
	name := values["name"]
	mgr, ctx := m.mgr, m.ctx
	m.confirm = &confirmState{
		title: "Delete cookie?",
		body:  fmt.Sprintf("%s  %s  %s", domain, path, name),
		action: func() tea.Msg {
			cookies, err := mgr.DeleteCookie(ctx, domain, path, name)
			return cookiesMsg{cookies: cookies, err: err}
		},
	}
	return nil
}

func (m model) formValues() map[string]string {
	values := make(map[string]string, len(m.formInputs))
	for _, input := range m.formInputs {
		label := strings.TrimSuffix(input.Prompt, ": ")
		values[label] = input.Value()
	}
	return values
}

func (m *model) setScreen(s screen) {
	m.err = "" // switching screens starts fresh; drop the previous error
	m.screen = s
	m.focus = focusFiles
	m.commandOpen = false
	m.historyDetailMode = false
	m.initFormForScreen(s)
	m.loadScreenState()
	m.preview.SetContent(m.secondaryContent())
}

// loadScreenState refreshes the cached status DTOs for the active secondary
// screen. It runs on the Update path (never during View), which is what keeps
// rendering pure and cheap.
func (m *model) loadScreenState() {
	switch m.screen {
	case screenStress:
		m.stress = m.mgr.StressStatus(m.ctx)
		m.stressResult, _ = m.mgr.StressResult(m.ctx)
	case screenMock:
		m.mock = m.mgr.MockStatus(m.ctx)
	case screenRecord:
		m.record = m.mgr.RecordStatus(m.ctx)
	}
}

func (m *model) initFormForScreen(s screen) {
	m.formMode = screenNames[s]
	m.formFocus = 0
	m.formActive = false
	switch s {
	case screenStress:
		m.formInputs = []textinput.Model{
			m.newInput("duration", "30s", 12),
			m.newInput("rate", "5", 10),
			m.newInput("vus", "0", 8),
			m.newInput("max vus", "0", 8),
		}
	case screenMock:
		m.formInputs = []textinput.Model{
			m.newInput("port", "3000", 10),
			m.newInput("delay", "", 12),
		}
	case screenContract:
		m.formInputs = []textinput.Model{
			m.newInput("provider URL", "http://localhost:3000", 44),
			m.newInput("state handler", "", 44),
		}
	case screenRecord:
		m.formInputs = []textinput.Model{
			m.newInput("target URL", "http://localhost:3000", 44),
			m.newInput("port", "8081", 10),
			m.newInput("deduplicate", "true", 10),
		}
	case screenImport:
		m.formInputs = []textinput.Model{
			m.newInput("format", "curl", 12),
			m.newInput("input", "curl https://example.com", 64),
			m.newInput("base URL", "", 44),
		}
	case screenCookies:
		m.formInputs = []textinput.Model{
			m.newInput("domain", "example.com", 30),
			m.newInput("path", "/", 12),
			m.newInput("name", "session", 24),
			m.newInput("value", "", 48),
		}
	case screenSettings:
		timeout := strconv.Itoa(m.config.Timeout)
		if timeout == "0" {
			timeout = ""
		}
		retries := strconv.Itoa(m.config.Retries)
		if retries == "0" {
			retries = ""
		}
		concurrency := strconv.Itoa(m.config.Concurrency)
		if concurrency == "0" {
			concurrency = ""
		}
		m.formInputs = []textinput.Model{
			m.newInput("default env", m.workspace.Environment, 24),
			m.newInput("timeout ms", timeout, 12),
			m.newInput("retries", retries, 8),
			m.newInput("concurrency", concurrency, 8),
			m.newInput("set var env", m.workspace.Environment, 24),
			m.newInput("set var key", "", 24),
			m.newInput("set var value", "", 48),
		}
	default:
		m.formInputs = nil
	}
	m.focusForm(false)
}

func (m *model) newInput(label, value string, width int) textinput.Model {
	input := textinput.New()
	input.Prompt = label + ": "
	input.CharLimit = 4096
	// Clamp wide fields to the terminal so they don't overflow on narrow widths
	// (the workspace already switches to a single-pane layout under 78 cols).
	if m.width > 0 {
		width = min(width, max(20, m.width-8))
	}
	input.SetWidth(width)
	input.SetValue(value)
	input.Blur()
	return input
}

func (m *model) focusForm(active bool) tea.Cmd {
	m.formActive = active
	var cmd tea.Cmd
	for i := range m.formInputs {
		m.formInputs[i].Blur()
	}
	if active && len(m.formInputs) > 0 {
		if m.formFocus < 0 || m.formFocus >= len(m.formInputs) {
			m.formFocus = 0 // guard against a stale index from a differently-sized form
		}
		cmd = m.formInputs[m.formFocus].Focus()
	}
	return cmd
}

func (m *model) moveFormFocus(delta int) {
	if len(m.formInputs) == 0 {
		return
	}
	m.formInputs[m.formFocus].Blur()
	m.formFocus = (m.formFocus + delta + len(m.formInputs)) % len(m.formInputs)
	_ = m.formInputs[m.formFocus].Focus()
}

func (m *model) nextFocus() {
	m.focus++
	if m.focus > focusResponse {
		m.focus = focusFiles
	}
	m.syncFocus()
}

func (m *model) prevFocus() {
	if m.focus == focusFiles {
		m.focus = focusResponse
	} else {
		m.focus--
	}
	m.syncFocus()
}

func (m *model) syncFocus() {
	m.requests.Blur()
	m.source.Blur()
	if m.focus == focusRequests {
		m.requests.Focus()
	}
	if m.focus == focusSource && m.editing {
		_ = m.source.Focus()
	}
}

func (m *model) resize() {
	w, h := max(40, m.width), max(12, m.height)
	sidebar := clamp(w/4, 24, 36)
	mainW := max(20, w-sidebar-4)
	bodyH := max(6, h-4)
	m.filesList.SetSize(sidebar-2, bodyH-2)
	m.palette.SetSize(clamp(w-8, 32, 70), clamp(h-8, 10, 24))
	m.envList.SetSize(clamp(w/3, 28, 50), clamp(h-8, 8, 18))
	m.themeList.SetSize(clamp(w/3, 28, 50), clamp(h-8, 8, 18))
	m.historyList.SetSize(max(20, w-6), max(6, bodyH-3))
	m.searchInput.SetWidth(clamp(w-12, 30, 70))
	m.searchList.SetSize(clamp(w-10, 32, 72), clamp(h-10, 6, 18))
	m.source.SetWidth(mainW/2 - 4)
	m.source.SetHeight(max(1, bodyH-10))
	m.respView.setSize(mainW/2-4, max(1, bodyH-10))
	m.preview.SetWidth(w - 4)
	m.preview.SetHeight(max(1, bodyH-2))
	m.requests.SetWidth(mainW - 2)
	m.requests.SetHeight(8)
	m.headers.SetWidth(mainW/2 - 4)
	m.headers.SetHeight(7)
	m.assertions.SetWidth(mainW/2 - 4)
	m.assertions.SetHeight(7)
}

func (m *model) refreshFileList() {
	items := make([]list.Item, 0, len(m.files))
	for _, f := range m.files {
		items = append(items, fileItem{path: f.RelativePath, name: f.Name, count: f.RequestCount})
	}
	_ = m.filesList.SetItems(items)
	for i, f := range m.files {
		if f.RelativePath == m.selected {
			m.filesList.Select(i)
			break
		}
	}
}

// refreshEnvList rebuilds the environment switcher items from m.envs, marking
// and pre-selecting the active environment.
func (m *model) refreshEnvList() {
	active := m.workspace.Environment
	if active == "" {
		active = m.config.DefaultEnvironment
	}
	items := make([]list.Item, 0, len(m.envs))
	selected := 0
	for i, e := range m.envs {
		it := envItem{name: e.Name, vars: len(e.Variables), active: e.Name == active}
		items = append(items, it)
		if it.active {
			selected = i
		}
	}
	_ = m.envList.SetItems(items)
	if len(items) > 0 {
		m.envList.Select(selected)
	}
}

func (m *model) refreshRequestTables() {
	var rows []table.Row
	var headerRows []table.Row
	var assertionRows []table.Row
	if m.parsed != nil {
		for _, r := range m.parsed.Requests {
			name := r.Name
			if name == "" {
				name = fmt.Sprintf("line %d", r.Line)
			}
			rows = append(rows, table.Row{name, r.Method, r.URL})
		}
		if len(m.parsed.Requests) > 0 {
			r := m.parsed.Requests[0]
			for _, h := range r.Headers {
				headerRows = append(headerRows, table.Row{h.Key, h.Value})
			}
			for _, a := range r.Assertions {
				assertionRows = append(assertionRows, table.Row{a.Subject, a.Operator, fmt.Sprint(a.Expected)})
			}
		}
	}
	m.requests.SetRows(rows)
	m.headers.SetRows(headerRows)
	m.assertions.SetRows(assertionRows)
}

func (m *model) refreshResultViews() {
	if m.lastResult == nil {
		return
	}
	m.respView.setResult(m.lastResult)
	var headerRows []table.Row
	var assertionRows []table.Row
	if len(m.lastResult.Results) > 0 {
		rr := m.lastResult.Results[len(m.lastResult.Results)-1]
		if rr.Response != nil {
			for _, k := range sortedStringKeys(rr.Response.Headers) {
				headerRows = append(headerRows, table.Row{k, rr.Response.Headers[k]})
			}
		}
		for _, a := range rr.Assertions {
			state := "fail"
			if a.Passed {
				state = "pass"
			}
			assertionRows = append(assertionRows, table.Row{a.Subject, a.Operator, state + " " + fmt.Sprint(a.Expected)})
		}
	}
	m.headers.SetRows(headerRows)
	m.assertions.SetRows(assertionRows)
}

func (m *model) handleEvent(ev clientmgr.Event) {
	switch ev.Type {
	case "file_changed":
		m.status = "file event received"
	case "request_progress":
		if p, ok := ev.Payload.(clientmgr.RequestProgress); ok {
			m.progress = p
			if p.Total > 0 {
				_ = m.progressBar.SetPercent(float64(p.Index) / float64(p.Total))
			}
		}
	case "stress_update":
		if s, ok := ev.Payload.(clientmgr.StressMetrics); ok {
			stats := s.Stats
			m.stress = clientmgr.StressStatusDTO{Running: s.Running, Elapsed: s.Elapsed, Stats: &stats}
			if !s.Running {
				m.stressResult, _ = m.mgr.StressResult(m.ctx)
			}
			if m.screen == screenStress {
				m.preview.SetContent(m.stressContent())
			}
		}
	case "mock_request":
		m.mock = m.mgr.MockStatus(m.ctx)
		if m.screen == screenMock {
			m.preview.SetContent(m.mockContent())
		}
	}
}

func (m model) render() string {
	top := m.topbar()
	var body string
	if m.screen == screenWorkspace {
		body = m.workspaceView()
	} else {
		body = m.secondaryView()
	}
	status := m.statusbar()
	content := lipgloss.JoinVertical(lipgloss.Left, top, m.navstrip(), body, status)
	frame := m.styles.app.Width(m.width).Height(m.height).Render(content)
	if m.commandOpen {
		// Like every other overlay, composite over the background (overlayCenter)
		// rather than blanking it with lipgloss.Place.
		pal := m.styles.panelHot.Width(m.palette.Width() + 2).Render(m.palette.View())
		frame = overlayCenter(frame, m.width, m.height, pal)
	}
	if m.envOpen {
		box := m.styles.panelHot.Render(m.envList.View())
		frame = overlayCenter(frame, m.width, m.height, box)
	}
	if m.themeOpen {
		box := m.styles.panelHot.Render(m.themeList.View())
		frame = overlayCenter(frame, m.width, m.height, box)
	}
	if m.searchOpen {
		count := m.styles.muted.Render(fmt.Sprintf("%d results", len(m.searchResults)))
		if strings.TrimSpace(m.searchInput.Value()) == "" {
			count = m.styles.muted.Render("type to search")
		}
		hint := m.styles.help.Render("enter select · esc cancel")
		body := lipgloss.JoinVertical(lipgloss.Left, m.searchInput.View(), count, "", m.searchList.View(), "", hint)
		frame = overlayCenter(frame, m.width, m.height, m.styles.panelHot.Render(body))
	}
	if m.showHelp {
		frame = overlayCenter(frame, m.width, m.height, m.helpOverlay())
	}
	if m.confirm != nil {
		frame = overlayCenter(frame, m.width, m.height, m.confirm.view(m.styles))
	}
	if m.prompt != nil {
		frame = overlayCenter(frame, m.width, m.height, m.prompt.view(m.styles))
	}
	if !m.toasts.empty() {
		frame = overlayBottomRight(frame, m.width, m.height, m.toasts.view(m.styles))
	}
	return frame
}

func (m model) topbar() string {
	active := strings.ToUpper(screenNames[m.screen])
	dirty := ""
	if m.dirty {
		dirty = m.styles.warn.Render(" modified")
	}
	left := m.styles.top.Render("hitspec studio  " + m.styles.tag.Render(active) + dirty)
	right := m.styles.top.Render(m.workspace.Environment + "  " + m.selected)
	if m.width <= lipgloss.Width(left)+lipgloss.Width(right)+2 {
		return m.styles.top.Width(m.width).Render("hitspec studio " + active)
	}
	gap := strings.Repeat(" ", max(1, m.width-lipgloss.Width(left)-lipgloss.Width(right)))
	return left + gap + right
}

func (m model) statusbar() string {
	helpView := m.hints()
	progress := ""
	if m.progress.Total > 0 {
		progress = fmt.Sprintf("  %s %d/%d", m.progress.RequestName, m.progress.Index, m.progress.Total)
	}
	state := m.status
	if m.err != "" {
		// Truncate to the space left of the hints/progress so a long error never
		// overflows the right edge or collides with the hint bar (counts runes
		// before styling, since ANSI would corrupt the width math).
		budget := max(10, m.width-lipgloss.Width(helpView)-lipgloss.Width(progress)-4)
		state = m.styles.danger.Render(truncateText(m.err, budget))
	}
	if m.loading {
		state = m.spinner.View() + " " + state
	}
	line := state + progress
	spacing := max(1, m.width-lipgloss.Width(line)-lipgloss.Width(helpView)-2)
	return m.styles.status.Width(m.width).Render(line + strings.Repeat(" ", spacing) + helpView)
}

func (m model) workspaceView() string {
	if len(m.files) == 0 {
		return m.welcomeView()
	}
	if m.width < 78 {
		return m.narrowWorkspace()
	}
	sidebarW := clamp(m.width/4, 24, 36)
	bodyW := max(30, m.width-sidebarW-2)
	bodyH := max(6, m.height-4)
	sidebar := m.panel("files", m.filesList.View(), sidebarW, bodyH, m.focus == focusFiles)
	var main string
	if m.width < 120 {
		main = m.mediumMain(bodyW, bodyH)
	} else {
		main = m.wideMain(bodyW, bodyH)
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, sidebar, main)
}

func (m model) narrowWorkspace() string {
	title := "source"
	content := m.source.View()
	if m.focus == focusFiles {
		title = "files"
		content = m.filesList.View()
	} else if m.focus == focusRequests {
		title = "requests"
		content = m.requests.View()
	} else if m.focus == focusResponse {
		title = "response"
		content = m.respView.view()
	}
	return m.panel(title, content, m.width, max(6, m.height-4), true)
}

func (m model) mediumMain(w, h int) string {
	var content string
	switch m.focus {
	case focusRequests:
		content = m.requests.View()
	case focusResponse:
		content = m.respView.view()
	default:
		content = m.source.View()
	}
	meta := lipgloss.JoinHorizontal(lipgloss.Top,
		m.panel("headers", m.headers.View(), w/2, 8, false),
		m.panel("assertions", m.assertions.View(), w/2, 8, false),
	)
	return lipgloss.JoinVertical(lipgloss.Left,
		m.panel("requests", m.requests.View(), w, 9, m.focus == focusRequests),
		m.panel("main", content, w, h-19, m.focus == focusSource || m.focus == focusResponse),
		meta,
	)
}

func (m model) wideMain(w, h int) string {
	leftW := w / 2
	rightW := w - leftW
	requests := m.panel("requests", m.requests.View(), w, 9, m.focus == focusRequests)
	source := m.panel("source", m.source.View(), leftW, h-18, m.focus == focusSource)
	response := m.panel("response", m.respView.view(), rightW, h-18, m.focus == focusResponse)
	tables := lipgloss.JoinHorizontal(lipgloss.Top,
		m.panel("headers", m.headers.View(), leftW, 8, false),
		m.panel("assertions", m.assertions.View(), rightW, 8, false),
	)
	return lipgloss.JoinVertical(lipgloss.Left, requests, lipgloss.JoinHorizontal(lipgloss.Top, source, response), tables)
}

func (m model) secondaryView() string {
	// preview is kept current on the Update path; View must stay pure (no I/O).
	if m.screen == screenHistory {
		var content, hint string
		if m.historyDetailMode {
			content = m.preview.View()
			hint = m.styles.help.Render("esc back · ctrl+r refresh")
		} else {
			content = m.historyList.View()
			hint = m.styles.help.Render("enter details · D delete · ctrl+r refresh")
		}
		body := lipgloss.JoinVertical(lipgloss.Left, content, hint)
		return m.panel("history", body, m.width, max(6, m.height-4), true)
	}
	return m.panel(screenNames[m.screen], m.preview.View(), m.width, max(6, m.height-4), true)
}

func (m model) secondaryContent() string {
	var content string
	switch m.screen {
	case screenStress:
		content = m.stressContent()
	case screenMock:
		content = m.mockContent()
	case screenContract:
		content = m.contractContent()
	case screenRecord:
		content = m.recordContent()
	case screenHistory:
		content = m.historyContent()
	case screenImport:
		content = m.importContent()
	case screenCookies:
		content = m.cookiesContent()
	case screenSettings:
		content = m.settingsContent()
	default:
		content = ""
	}
	if len(m.formInputs) == 0 {
		return content
	}
	return lipgloss.JoinVertical(lipgloss.Left, m.formView(), "", content, "", m.secondaryHelp())
}

func (m model) formView() string {
	var lines []string
	for i, input := range m.formInputs {
		line := input.View()
		if m.formActive && i == m.formFocus {
			line = m.styles.selection.Render(line)
		}
		lines = append(lines, line)
	}
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func (m model) secondaryHelp() string {
	action := "enter submit"
	switch m.screen {
	case screenStress:
		action = "enter/s start stress   x stop"
	case screenMock:
		action = "enter/s start mock   x stop"
	case screenContract:
		action = "enter/s verify selected file"
	case screenRecord:
		action = "enter/s start recorder   x stop   E export"
	case screenImport:
		action = "enter/s import into scratch file"
	case screenCookies:
		action = "enter/s add cookie   D delete matching cookie"
	case screenSettings:
		action = "enter/s save config & set var"
	}
	return m.styles.help.Render("e edit fields   " + action + "   esc cancel")
}

func (m model) panel(title, content string, width, height int, active bool) string {
	style := m.styles.panel
	if active {
		style = m.styles.panelHot
	}
	header := m.styles.title.Render(title)
	return style.Width(max(8, width-2)).Height(max(3, height-2)).Render(lipgloss.JoinVertical(lipgloss.Left, header, content))
}

func (m model) fileSummary() string {
	if m.parsed == nil {
		return "No file loaded."
	}
	var sb strings.Builder
	sb.WriteString(m.styles.accent.Render(m.selected))
	sb.WriteString("\n\n")
	fmt.Fprintf(&sb, "Variables: %d\nRequests: %d\n", len(m.parsed.Variables), len(m.parsed.Requests))
	for _, r := range m.parsed.Requests {
		name := r.Name
		if name == "" {
			name = fmt.Sprintf("line %d", r.Line)
		}
		fmt.Fprintf(&sb, "\n%s %s\n", m.styles.methodBadge(r.Method).Render(r.Method), name)
		sb.WriteString(m.styles.muted.Render(r.URL))
		sb.WriteByte('\n')
	}
	return sb.String()
}

// maxResponseBodyRender bounds how much of a body the response pane renders so a
// huge payload never stalls the highlighter or the viewport.
const maxResponseBodyRender = 24000

// humanBytes formats a byte count in a compact, human-readable form.
func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for x := n / unit; x >= unit; x /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}

func (m model) stressContent() string {
	status := m.stress
	if status.Stats == nil {
		if m.stressResult != nil {
			return m.stressResultContent()
		}
		return "No stress test running.\n\nUse the form (e edit) or the command palette to start a load test.\nQuick start: 30s at 5 RPS on the selected file."
	}
	stats := status.Stats
	live := fmt.Sprintf(
		"Running: %v\nElapsed: %.1fs\n\n%s\n\nRequests: %d\nSuccess: %d\nErrors: %d\nRPS: %.2f\nP50/P95/P99: %.1f / %.1f / %.1f ms\nError rate: %.2f%%\nActive VUs: %d\n",
		status.Running,
		status.Elapsed,
		m.progressBar.ViewAs(min(1, status.Elapsed/30.0)),
		stats.Total,
		stats.Success,
		stats.Errors,
		stats.RPS,
		stats.P50Ms,
		stats.P95Ms,
		stats.P99Ms,
		stats.ErrorRate*100,
		stats.ActiveVUs,
	)
	if !status.Running && m.stressResult != nil {
		return live + "\n" + m.stressResultContent()
	}
	return live
}

// stressResultContent renders the full completed stress report (percentiles +
// per-request breakdown) from the cached StressResult.
func (m model) stressResultContent() string {
	r := m.stressResult
	if r == nil {
		return ""
	}
	var sb strings.Builder
	sb.WriteString(m.styles.title.Render("Last result") + "\n")
	fmt.Fprintf(&sb, "Duration %.1fs   Total %d   %s success   %s errors   %d timeouts\n",
		r.DurationMs/1000.0, r.Total,
		m.styles.success.Render(fmt.Sprint(r.Success)),
		m.styles.danger.Render(fmt.Sprint(r.Errors)),
		r.Timeouts)
	fmt.Fprintf(&sb, "RPS %.1f   success rate %.1f%%   error rate %.1f%%\n", r.RPS, r.SuccessRate*100, r.ErrorRate*100)
	fmt.Fprintf(&sb, "Latency  min %.1f · mean %.1f · p50 %.1f · p95 %.1f · p99 %.1f · max %.1f ms\n",
		r.MinMs, r.MeanMs, r.P50Ms, r.P95Ms, r.P99Ms, r.MaxMs)
	if len(r.Breakdown) > 0 {
		sb.WriteString("\n" + m.styles.title.Render("Per request") + "\n")
		for _, b := range r.Breakdown {
			fmt.Fprintf(&sb, "%-24s total %d  ok %d  err %d  p95 %.1fms\n",
				truncateText(b.Name, 24), b.Total, b.Success, b.Errors, b.P95Ms)
		}
	}
	return sb.String()
}

func (m model) mockContent() string {
	status := m.mock
	if !status.Running && len(status.Routes) == 0 {
		return "No mock server running.\n\nPress e to edit the form (port, delay), then enter to start it.\nRoutes are served from the selected .http file's requests."
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "Running: %v\nPort: %d\nRoutes: %d\n\n", status.Running, status.Port, len(status.Routes))
	for _, r := range status.Routes {
		fmt.Fprintf(&sb, "%-7s %-40s %d %s\n", r.Method, r.Path, r.StatusCode, r.ContentType)
	}
	return sb.String()
}

func (m model) contractContent() string {
	if len(m.contracts) == 0 {
		return "No contract verification result loaded.\nSelect a file, edit provider URL, then press enter to verify."
	}
	var sb strings.Builder
	for _, result := range m.contracts {
		fmt.Fprintf(&sb, "%s  pass:%d fail:%d skip:%d %.0fms\n", result.File, result.Passed, result.Failed, result.Skipped, result.Duration)
		for _, interaction := range result.Results {
			state := m.styles.danger.Render("FAIL")
			if interaction.Passed {
				state = m.styles.success.Render("PASS")
			}
			fmt.Fprintf(&sb, "  %s %s %.0fms", state, interaction.Name, interaction.Duration)
			if interaction.State != "" {
				sb.WriteString("  state=" + interaction.State)
			}
			if interaction.Error != "" {
				sb.WriteString("  " + interaction.Error)
			}
			sb.WriteByte('\n')
		}
	}
	return sb.String()
}

func (m model) recordContent() string {
	status := m.record
	if !status.Running && status.Count == 0 {
		return "No recording proxy running.\n\nPress e to set the target URL and proxy port, then enter to start it.\nPoint your client at http://localhost:PORT to capture traffic, then E to export."
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "Running: %v\nTarget: %s\nPort: %d\nCaptured: %d\n\n", status.Running, status.TargetURL, status.Port, status.Count)
	for _, r := range status.Recordings {
		fmt.Fprintf(&sb, "%-7s %-40s %d %.0fms\n", r.Method, r.Path, r.StatusCode, r.Duration)
	}
	return sb.String()
}

func (m model) historyContent() string {
	if len(m.history.Runs) == 0 {
		return "No persistent history loaded. Press 6 or ctrl+r to refresh."
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "Runs: %d of %d\n\n", len(m.history.Runs), m.history.Total)
	for _, run := range m.history.Runs {
		fmt.Fprintf(&sb, "#%d %s %s  pass:%d fail:%d skip:%d  %.0fs\n",
			run.ID, run.FilePath, run.StartedAt, run.Passed, run.Failed, run.Skipped, float64(run.DurationMs)/1000.0)
	}
	return sb.String()
}

func (m model) importContent() string {
	return "Import supports cURL, Insomnia, OpenAPI, and Postman through the manager.\n\nThe command palette provides quick import actions; imported content opens in a scratch file for review before saving."
}

func (m model) cookiesContent() string {
	if len(m.cookies) == 0 {
		return "No local cookies stored.\nCookies captured from Set-Cookie headers are saved here for inspection only."
	}
	var sb strings.Builder
	for _, c := range m.cookies {
		fmt.Fprintf(&sb, "%s\t%s\t%s=%s\n", c.Domain, c.Path, c.Name, c.Value)
	}
	return sb.String()
}

func (m model) settingsContent() string {
	var sb strings.Builder
	sb.WriteString("Configuration\n\n")
	fmt.Fprintf(&sb, "Default environment: %s\nTimeout: %d\nRetries: %d\nProxy: %s\nConcurrency: %d\n",
		m.config.DefaultEnvironment, m.config.Timeout, m.config.Retries, m.config.Proxy, m.config.Concurrency)
	sb.WriteString("\nEnvironments\n\n")
	active := m.workspace.Environment
	if active == "" {
		active = m.config.DefaultEnvironment
	}
	for _, env := range m.envs {
		marker := "  "
		if env.Name == active {
			marker = m.styles.accent.Render("● ")
		}
		fmt.Fprintf(&sb, "%s%s  (%d variables)\n", marker, env.Name, len(env.Variables))
		for _, k := range sortedAnyKeys(env.Variables) {
			fmt.Fprintf(&sb, "    %s = %v\n", m.styles.muted.Render(k), env.Variables[k])
		}
	}
	return sb.String()
}
