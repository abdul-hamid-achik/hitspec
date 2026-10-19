package env

import (
	"encoding/json"
	"testing"
)

// B1: the REST Client idiom `@baseUrl = {{baseUrl}}` makes the file default
// delegate to the lower-precedence layers (environment, dotenv, OS env).

func TestResolver_SelfRefDelegatesToEnvironment(t *testing.T) {
	r := NewResolver()
	r.SetVariables(map[string]any{"baseUrl": "http://env.example"}) // hitspec.yaml environment
	r.SetVariable("baseUrl", "{{baseUrl}}")                         // @baseUrl = {{baseUrl}}
	if got := r.Resolve("{{baseUrl}}/health"); got != "http://env.example/health" {
		t.Errorf("got %q, want http://env.example/health", got)
	}
}

func TestResolver_SelfRefWithPrefixDelegates(t *testing.T) {
	r := NewResolver()
	r.SetVariables(map[string]any{"api": "http://env.example"})
	r.SetVariable("api", "{{api}}/v1")
	if got := r.Resolve("{{api}}/users"); got != "http://env.example/v1/users" {
		t.Errorf("got %q", got)
	}
}

func TestResolver_SelfRefDelegatesToDotenv(t *testing.T) {
	r := NewResolver()
	r.dotenv["P0B_DOT"] = "http://dotenv.example"
	r.SetVariable("P0B_DOT", "{{P0B_DOT}}")
	if got := r.Resolve("{{P0B_DOT}}"); got != "http://dotenv.example" {
		t.Errorf("got %q", got)
	}
}

func TestResolver_SelfRefDelegatesToOSEnv(t *testing.T) {
	t.Setenv("P0B_OS_BASE", "http://os.example")
	r := NewResolver()
	r.SetVariable("P0B_OS_BASE", "{{P0B_OS_BASE}}")
	if got := r.Resolve("{{P0B_OS_BASE}}"); got != "http://os.example" {
		t.Errorf("got %q", got)
	}
}

// The environment wins over dotenv and OS env, as for any other variable.
func TestResolver_SelfRefPrefersEnvironmentOverDotenvAndOS(t *testing.T) {
	t.Setenv("P0B_ORDER", "os")
	r := NewResolver()
	r.dotenv["P0B_ORDER"] = "dotenv"
	r.SetVariables(map[string]any{"P0B_ORDER": "yaml"})
	r.SetVariable("P0B_ORDER", "{{P0B_ORDER}}")
	if got := r.Resolve("{{P0B_ORDER}}"); got != "yaml" {
		t.Errorf("got %q, want yaml", got)
	}
}

// Without a lower layer the old behaviour stays: literal, no hang.
func TestResolver_SelfRefWithoutLowerLayerStaysLiteral(t *testing.T) {
	r := NewResolver()
	r.SetVariable("p0bNoLower", "{{p0bNoLower}}")
	if got := r.Resolve("{{p0bNoLower}}"); got != "{{p0bNoLower}}" {
		t.Errorf("got %q", got)
	}
}

// Loading a second file resets the environment layer; a stale shadow from the
// first file must not leak.
func TestResolver_SelfRefReloadDoesNotLeakStaleShadow(t *testing.T) {
	r := NewResolver()
	r.SetVariables(map[string]any{"baseUrl": "http://one"})
	r.SetVariable("baseUrl", "{{baseUrl}}")
	r.SetVariables(map[string]any{"baseUrl": "http://two"})
	r.SetVariable("baseUrl", "{{baseUrl}}")
	if got := r.Resolve("{{baseUrl}}"); got != "http://two" {
		t.Errorf("got %q, want http://two", got)
	}
	r.SetVariable("baseUrl", "http://plain")
	if got := r.Resolve("{{baseUrl}}"); got != "http://plain" {
		t.Errorf("got %q, want http://plain", got)
	}
}

// Real cycles between distinct variables are still reported, not resolved.
func TestResolver_RealCycleStillUnresolved(t *testing.T) {
	r := NewResolver()
	var warnings []string
	r.SetWarnFunc(func(format string, args ...any) { warnings = append(warnings, format) })
	r.SetVariable("cycA", "{{cycB}}")
	r.SetVariable("cycB", "{{cycA}}")
	got := r.Resolve("{{cycA}}")
	if got != "{{cycA}}" && got != "{{cycB}}" {
		t.Errorf("cycle resolved to %q, want a literal reference", got)
	}
	found := false
	for _, w := range warnings {
		if w == "circular variable reference: %s" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a circular reference warning, got %v", warnings)
	}
}

// B2: captured (response-derived) values are inserted literally.

func TestResolver_CapturedValueNotReExpanded(t *testing.T) {
	t.Setenv("P0B_SECRET", "leak")
	r := NewResolver()
	r.SetVariable("home", "/home/me")
	r.SetCapture("login", "v", "{{$env(P0B_SECRET)}}")
	r.SetCapture("login", "w", "{{home}}")

	for in, want := range map[string]string{
		"{{v}}":       "{{$env(P0B_SECRET)}}",
		"{{login.v}}": "{{$env(P0B_SECRET)}}",
		"{{w}}":       "{{home}}",
		"x={{w}}&y":   "x={{home}}&y",
	} {
		if got := r.Resolve(in); got != want {
			t.Errorf("Resolve(%q) = %q, want %q", in, got, want)
		}
	}
}

// A file variable that references a capture gets the capture literally, while
// the variable's own definition is still expanded.
func TestResolver_VariableReferencingCaptureIsLiteral(t *testing.T) {
	r := NewResolver()
	r.SetVariable("home", "/home/me")
	r.SetCapture("a", "tok", "{{home}}")
	r.SetVariable("auth", "Bearer {{tok}} for {{home}}")
	if got := r.Resolve("{{auth}}"); got != "Bearer {{home}} for /home/me" {
		t.Errorf("got %q", got)
	}
}

// B3: non-string values interpolate as their JSON text.

func TestResolver_InterpolatesValuesAsJSONText(t *testing.T) {
	r := NewResolver()
	r.SetCapture("q", "int", json.Number("1000000"))
	r.SetCapture("q", "big", json.Number("123456789012"))
	r.SetCapture("q", "frac", json.Number("1.5"))
	r.SetCapture("q", "f64int", float64(1000000))
	r.SetCapture("q", "f64big", float64(123456789012))
	r.SetCapture("q", "f64frac", 0.25)
	r.SetCapture("q", "yes", true)
	r.SetCapture("q", "no", false)
	r.SetCapture("q", "nil", nil)
	r.SetCapture("q", "obj", map[string]any{"b": []any{json.Number("1"), "x"}, "a": json.Number("2")})
	r.SetCapture("q", "arr", []any{"a", json.Number("1"), nil})
	r.SetCapture("q", "html", map[string]any{"k": "<&>"})
	r.SetCapture("q", "i64", int64(42))
	r.SetCapture("q", "str", "plain")

	want := map[string]string{
		"{{int}}":     "1000000",
		"{{big}}":     "123456789012",
		"{{frac}}":    "1.5",
		"{{f64int}}":  "1000000",
		"{{f64big}}":  "123456789012",
		"{{f64frac}}": "0.25",
		"{{yes}}":     "true",
		"{{no}}":      "false",
		"{{nil}}":     "null",
		"{{obj}}":     `{"a":2,"b":[1,"x"]}`,
		"{{arr}}":     `["a",1,null]`,
		"{{html}}":    `{"k":"<&>"}`,
		"{{i64}}":     "42",
		"{{str}}":     "plain",
	}
	for in, w := range want {
		if got := r.Resolve(in); got != w {
			t.Errorf("Resolve(%q) = %q, want %q", in, got, w)
		}
	}
}
