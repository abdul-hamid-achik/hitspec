package tui

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func TestToastPushAndExpire(t *testing.T) {
	c := newToastCenter()
	c, cmd := c.push(toastInfo, "hello")
	if cmd == nil {
		t.Fatal("push should return an expiry command")
	}
	if len(c.items) != 1 {
		t.Fatalf("want 1 toast, got %d", len(c.items))
	}
	id := c.items[0].id
	c = c.expire(id)
	if !c.empty() {
		t.Fatalf("expire should remove the toast, got %d", len(c.items))
	}
}

func TestToastRespectsMax(t *testing.T) {
	c := newToastCenter()
	for i := 0; i < toastMax+2; i++ {
		c, _ = c.push(toastInfo, fmt.Sprintf("t%d", i))
	}
	if len(c.items) != toastMax {
		t.Fatalf("want %d toasts, got %d", toastMax, len(c.items))
	}
	if got := c.items[len(c.items)-1].text; got != fmt.Sprintf("t%d", toastMax+1) {
		t.Fatalf("newest toast = %q, want the last pushed", got)
	}
}

func TestToastView(t *testing.T) {
	s := newStyles(defaultPalette())
	c := newToastCenter()
	if c.view(s) != "" {
		t.Fatal("empty center should render nothing")
	}
	c, _ = c.push(toastSuccess, "saved file")
	if !strings.Contains(c.view(s), "saved file") {
		t.Fatalf("toast view missing text: %q", c.view(s))
	}
}

func TestTruncateText(t *testing.T) {
	if got := truncateText("short", 44); got != "short" {
		t.Fatalf("short text mutated: %q", got)
	}
	long := strings.Repeat("a", 100)
	got := truncateText(long, 10)
	if len([]rune(got)) != 10 || !strings.HasSuffix(got, "…") {
		t.Fatalf("truncateText wrong: len=%d val=%q", len([]rune(got)), got)
	}
}

func TestCopyErrorProducesToast(t *testing.T) {
	m := newModel(context.Background(), newTestManager(t), Options{})
	next, _ := m.Update(copyMsg{err: fmt.Errorf("boom")})
	nm := next.(model)
	if len(nm.toasts.items) != 1 {
		t.Fatalf("want 1 toast, got %d", len(nm.toasts.items))
	}
	if nm.toasts.items[0].severity != toastError {
		t.Fatalf("want error severity, got %v", nm.toasts.items[0].severity)
	}
}

func TestSimpleMsgProducesSuccessToast(t *testing.T) {
	m := newModel(context.Background(), newTestManager(t), Options{})
	next, _ := m.Update(simpleMsg{kind: "mock started"})
	nm := next.(model)
	found := false
	for _, tt := range nm.toasts.items {
		if tt.severity == toastSuccess && strings.Contains(tt.text, "mock started") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a success toast, got %+v", nm.toasts.items)
	}
}
