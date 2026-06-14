package tui

import (
	"strings"
	"testing"
)

func TestPrettyJSON(t *testing.T) {
	out := prettyJSON(`{"a":1,"b":[1,2]}`)
	if !strings.Contains(out, "\n") || !strings.Contains(out, `  "a": 1`) {
		t.Fatalf("prettyJSON did not indent: %q", out)
	}
	if got := prettyJSON("not json"); got != "not json" {
		t.Fatalf("prettyJSON mutated non-json: %q", got)
	}
	if got := prettyJSON("   "); got != "   " {
		t.Fatalf("prettyJSON mutated blank input: %q", got)
	}
}

func TestLexerName(t *testing.T) {
	cases := []struct{ ct, body, want string }{
		{"application/json", "", "json"},
		{"application/json; charset=utf-8", "", "json"},
		{"text/html; charset=utf-8", "", "html"},
		{"application/xml", "", "xml"},
		{"application/yaml", "", "yaml"},
		{"", `{"a":1}`, "json"},
		{"", "  [1,2,3]", "json"},
		{"", "<root/>", "xml"},
		{"text/plain", "hello", ""},
	}
	for _, c := range cases {
		if got := lexerName(c.ct, c.body); got != c.want {
			t.Errorf("lexerName(%q,%q)=%q want %q", c.ct, c.body, got, c.want)
		}
	}
}

func TestHighlightColorToggle(t *testing.T) {
	code := `{"a":1}`
	if plain := highlight(code, "json", false); plain != code {
		t.Fatalf("highlight(color=false) must be identity, got %q", plain)
	}
	colored := highlight(code, "json", true)
	if !strings.Contains(colored, "\x1b[") {
		t.Fatalf("highlight(color=true) should emit ANSI escapes, got %q", colored)
	}
	if got := highlight("", "json", true); got != "" {
		t.Fatalf("highlight of empty string should stay empty, got %q", got)
	}
}

func TestFormatBodyPrettyPrintsJSON(t *testing.T) {
	out := formatBody(`{"x":1}`, "application/json", false)
	if !strings.Contains(out, `"x": 1`) {
		t.Fatalf("formatBody did not pretty-print JSON: %q", out)
	}
	// Non-JSON is passed through unchanged when color is off.
	if got := formatBody("plain", "text/plain", false); got != "plain" {
		t.Fatalf("formatBody mutated plain text: %q", got)
	}
}

func TestHeaderValueCaseInsensitive(t *testing.T) {
	h := map[string]string{"Content-Type": "application/json"}
	if got := headerValue(h, "content-type"); got != "application/json" {
		t.Fatalf("headerValue case-insensitive lookup failed: %q", got)
	}
	if got := headerValue(h, "Content-Type"); got != "application/json" {
		t.Fatalf("headerValue exact lookup failed: %q", got)
	}
	if got := headerValue(nil, "x"); got != "" {
		t.Fatalf("headerValue(nil) should be empty, got %q", got)
	}
}

func TestHumanBytes(t *testing.T) {
	cases := []struct {
		in   int64
		want string
	}{
		{512, "512 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
	}
	for _, c := range cases {
		if got := humanBytes(c.in); got != c.want {
			t.Errorf("humanBytes(%d)=%q want %q", c.in, got, c.want)
		}
	}
}
