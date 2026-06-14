package tui

import "charm.land/bubbles/v2/key"

type screen int

const (
	screenWorkspace screen = iota
	screenStress
	screenMock
	screenContract
	screenRecord
	screenHistory
	screenImport
	screenCookies
	screenSettings
)

var screenNames = []string{
	"workspace",
	"stress",
	"mock",
	"contract",
	"record",
	"history",
	"import",
	"cookies",
	"settings",
}

type focusPane int

const (
	focusFiles focusPane = iota
	focusRequests
	focusSource
	focusResponse
)

type keyMap struct {
	Quit           key.Binding
	Help           key.Binding
	Palette        key.Binding
	EnvSwitch      key.Binding
	ThemeSwitch    key.Binding
	Search         key.Binding
	Tab            key.Binding
	BackTab        key.Binding
	Open           key.Binding
	Edit           key.Binding
	Save           key.Binding
	RunRequest     key.Binding
	RunFile        key.Binding
	Refresh        key.Binding
	NewFile        key.Binding
	DeleteFile     key.Binding
	GenerateSample key.Binding
	Workspace      key.Binding
	Stress         key.Binding
	Mock           key.Binding
	Contract       key.Binding
	Record         key.Binding
	History        key.Binding
	Import         key.Binding
	Cookies        key.Binding
	Settings       key.Binding
	Cancel         key.Binding
	Confirm        key.Binding
	FormNext       key.Binding
	FormPrev       key.Binding
}

func defaultKeyMap() keyMap {
	return keyMap{
		Quit:           key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
		Help:           key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		Palette:        key.NewBinding(key.WithKeys("ctrl+p"), key.WithHelp("ctrl+p", "palette")),
		EnvSwitch:      key.NewBinding(key.WithKeys("ctrl+e"), key.WithHelp("ctrl+e", "environment")),
		ThemeSwitch:    key.NewBinding(key.WithKeys("ctrl+t"), key.WithHelp("ctrl+t", "theme")),
		Search:         key.NewBinding(key.WithKeys("ctrl+f"), key.WithHelp("ctrl+f", "search")),
		Tab:            key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next pane")),
		BackTab:        key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("shift+tab", "prev pane")),
		Open:           key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "open/select")),
		Edit:           key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "edit source")),
		Save:           key.NewBinding(key.WithKeys("ctrl+s"), key.WithHelp("ctrl+s", "save")),
		RunRequest:     key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "run request")),
		RunFile:        key.NewBinding(key.WithKeys("R"), key.WithHelp("R", "run file")),
		Refresh:        key.NewBinding(key.WithKeys("ctrl+r"), key.WithHelp("ctrl+r", "refresh")),
		NewFile:        key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "new file")),
		DeleteFile:     key.NewBinding(key.WithKeys("D"), key.WithHelp("D", "delete file")),
		GenerateSample: key.NewBinding(key.WithKeys("g"), key.WithHelp("g", "generate sample")),
		Workspace:      key.NewBinding(key.WithKeys("1"), key.WithHelp("1", "workspace")),
		Stress:         key.NewBinding(key.WithKeys("2"), key.WithHelp("2", "stress")),
		Mock:           key.NewBinding(key.WithKeys("3"), key.WithHelp("3", "mock")),
		Contract:       key.NewBinding(key.WithKeys("4"), key.WithHelp("4", "contract")),
		Record:         key.NewBinding(key.WithKeys("5"), key.WithHelp("5", "record")),
		History:        key.NewBinding(key.WithKeys("6"), key.WithHelp("6", "history")),
		Import:         key.NewBinding(key.WithKeys("7"), key.WithHelp("7", "import")),
		Cookies:        key.NewBinding(key.WithKeys("8"), key.WithHelp("8", "cookies")),
		Settings:       key.NewBinding(key.WithKeys("9"), key.WithHelp("9", "settings")),
		Cancel:         key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel")),
		Confirm:        key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "confirm")),
		FormNext:       key.NewBinding(key.WithKeys("tab", "down"), key.WithHelp("tab", "next field")),
		FormPrev:       key.NewBinding(key.WithKeys("shift+tab", "up"), key.WithHelp("shift+tab", "prev field")),
	}
}
