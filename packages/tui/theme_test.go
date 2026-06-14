package tui

import (
	"context"
	"testing"
)

func TestPaletteByNameCaseInsensitive(t *testing.T) {
	for _, name := range []string{"Nord", "nord", "  NORD  ", "tokyo night", "Gruvbox Dark"} {
		if _, ok := paletteByName(name); !ok {
			t.Errorf("paletteByName(%q) = not found, want found", name)
		}
	}
	if _, ok := paletteByName("solarized"); ok {
		t.Error("paletteByName(solarized) = found, want not found")
	}
}

func TestNewModelHonorsThemeOption(t *testing.T) {
	m := newModel(context.Background(), newTestManager(t), Options{Theme: "Dracula"})
	if m.theme != "Dracula" {
		t.Fatalf("theme = %q, want Dracula", m.theme)
	}
	// An unknown theme falls back to the default rather than blanking out.
	m2 := newModel(context.Background(), newTestManager(t), Options{Theme: "bogus"})
	if m2.theme != defaultPalette().name {
		t.Fatalf("theme = %q, want default %q", m2.theme, defaultPalette().name)
	}
}

func TestApplyThemeSwitchesStyles(t *testing.T) {
	m := newModel(context.Background(), newTestManager(t), Options{})
	before := m.styles.accent.GetForeground()
	m.applyTheme("Gruvbox Dark")
	if m.theme != "Gruvbox Dark" {
		t.Fatalf("theme = %q, want Gruvbox Dark", m.theme)
	}
	if m.styles.accent.GetForeground() == before {
		t.Fatal("accent color did not change after theme switch")
	}
	// Unknown themes are ignored — state is left untouched.
	m.applyTheme("nope")
	if m.theme != "Gruvbox Dark" {
		t.Fatalf("unknown theme changed state to %q", m.theme)
	}
}

func TestMethodBadgeDistinctPerMethod(t *testing.T) {
	s := newStyles(defaultPalette())
	get := s.methodBadge("GET").GetBackground()
	post := s.methodBadge("post").GetBackground() // case-insensitive
	if get == post {
		t.Fatal("GET and POST badges should use distinct colors")
	}
	// Unknown method falls back to the generic accent tag.
	if s.methodBadge("BREW").GetBackground() != s.tag.GetBackground() {
		t.Fatal("unknown method should fall back to the accent tag")
	}
}

func TestBuildThemeItemsMarksActive(t *testing.T) {
	items := buildThemeItems("Dracula")
	if len(items) != len(themes) {
		t.Fatalf("items = %d, want %d", len(items), len(themes))
	}
	var activeCount int
	for _, it := range items {
		if ti, ok := it.(themeItem); ok && ti.active {
			activeCount++
			if ti.name != "Dracula" {
				t.Errorf("active item = %q, want Dracula", ti.name)
			}
		}
	}
	if activeCount != 1 {
		t.Fatalf("active items = %d, want exactly 1", activeCount)
	}
}
