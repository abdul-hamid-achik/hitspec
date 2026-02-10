package builtin

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"
)

func TestNewRegistry_HasAllDefaults(t *testing.T) {
	r := NewRegistry()
	expected := []string{
		"now", "timestamp", "timestampMs", "uuid", "random",
		"randomString", "randomEmail", "randomAlphanumeric",
		"base64", "base64Decode", "md5", "sha256",
		"urlEncode", "urlDecode", "date", "json", "env",
	}
	for _, name := range expected {
		if _, ok := r.Call(name + "()"); !ok {
			t.Errorf("expected function %q to be registered", name)
		}
	}
}

func TestRegistry_Call_UnknownFunction(t *testing.T) {
	r := NewRegistry()
	_, ok := r.Call("nonexistent()")
	if ok {
		t.Error("expected ok=false for unknown function")
	}
}

func TestRegistry_Call_NotFunctionSyntax(t *testing.T) {
	r := NewRegistry()
	_, ok := r.Call("not a function call")
	if ok {
		t.Error("expected ok=false for non-function syntax")
	}
}

func TestRegistry_Register(t *testing.T) {
	r := NewRegistry()
	r.Register("custom", func(args []string) any {
		return "custom-value"
	})
	val, ok := r.Call("custom()")
	if !ok {
		t.Fatal("custom function should be callable")
	}
	if val != "custom-value" {
		t.Errorf("got %v, want %q", val, "custom-value")
	}
}

func TestFuncNow(t *testing.T) {
	before := time.Now().UTC()
	val := funcNow(nil).(string)
	after := time.Now().UTC()

	parsed, err := time.Parse(time.RFC3339, val)
	if err != nil {
		t.Fatalf("invalid RFC3339: %v", err)
	}
	if parsed.Before(before.Add(-time.Second)) || parsed.After(after.Add(time.Second)) {
		t.Errorf("now() = %v, not in expected range", val)
	}
}

func TestFuncTimestamp(t *testing.T) {
	before := time.Now().Unix()
	val := funcTimestamp(nil).(int64)
	after := time.Now().Unix()

	if val < before || val > after {
		t.Errorf("timestamp = %d, not in range [%d, %d]", val, before, after)
	}
}

func TestFuncTimestampMs(t *testing.T) {
	before := time.Now().UnixMilli()
	val := funcTimestampMs(nil).(int64)
	after := time.Now().UnixMilli()

	if val < before || val > after+10 {
		t.Errorf("timestampMs = %d, not in range [%d, %d]", val, before, after)
	}
}

func TestFuncUUID(t *testing.T) {
	val := funcUUID(nil).(string)
	// UUID v4 format: xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx
	if len(val) != 36 {
		t.Errorf("uuid length = %d, want 36", len(val))
	}
	if val[14] != '4' {
		t.Errorf("uuid version byte = %c, want '4'", val[14])
	}
}

func TestFuncRandom(t *testing.T) {
	// Default range
	val := funcRandom(nil).(int)
	if val < 0 || val > 100 {
		t.Errorf("random() = %d, want [0,100]", val)
	}

	// Custom range
	val = funcRandom([]string{"10", "20"}).(int)
	if val < 10 || val > 20 {
		t.Errorf("random(10,20) = %d, want [10,20]", val)
	}
}

func TestFuncRandomString(t *testing.T) {
	// Default length
	val := funcRandomString(nil).(string)
	if len(val) != 16 {
		t.Errorf("randomString() length = %d, want 16", len(val))
	}

	// Custom length
	val = funcRandomString([]string{"8"}).(string)
	if len(val) != 8 {
		t.Errorf("randomString(8) length = %d, want 8", len(val))
	}
}

func TestFuncRandomEmail(t *testing.T) {
	val := funcRandomEmail(nil).(string)
	if !strings.Contains(val, "@") {
		t.Errorf("randomEmail() = %q, missing @", val)
	}
	if !strings.HasSuffix(val, ".com") {
		t.Errorf("randomEmail() = %q, should end with .com", val)
	}
}

func TestFuncRandomAlphanumeric(t *testing.T) {
	val := funcRandomAlphanumeric(nil).(string)
	if len(val) != 8 {
		t.Errorf("randomAlphanumeric() length = %d, want 8", len(val))
	}

	val = funcRandomAlphanumeric([]string{"4"}).(string)
	if len(val) != 4 {
		t.Errorf("randomAlphanumeric(4) length = %d, want 4", len(val))
	}
}

func TestFuncBase64(t *testing.T) {
	tests := []struct {
		args []string
		want string
	}{
		{nil, ""},
		{[]string{"hello"}, base64.StdEncoding.EncodeToString([]byte("hello"))},
		{[]string{""}, base64.StdEncoding.EncodeToString([]byte(""))},
	}
	for _, tt := range tests {
		got := funcBase64(tt.args).(string)
		if got != tt.want {
			t.Errorf("base64(%v) = %q, want %q", tt.args, got, tt.want)
		}
	}
}

func TestFuncBase64Decode(t *testing.T) {
	encoded := base64.StdEncoding.EncodeToString([]byte("hello world"))
	tests := []struct {
		args []string
		want string
	}{
		{nil, ""},
		{[]string{encoded}, "hello world"},
		{[]string{"!!!invalid!!!"}, ""},
	}
	for _, tt := range tests {
		got := funcBase64Decode(tt.args).(string)
		if got != tt.want {
			t.Errorf("base64Decode(%v) = %q, want %q", tt.args, got, tt.want)
		}
	}
}

func TestFuncMD5(t *testing.T) {
	tests := []struct {
		args []string
		want string
	}{
		{nil, ""},
		{[]string{"hello"}, "5d41402abc4b2a76b9719d911017c592"},
	}
	for _, tt := range tests {
		got := funcMD5(tt.args).(string)
		if got != tt.want {
			t.Errorf("md5(%v) = %q, want %q", tt.args, got, tt.want)
		}
	}
}

func TestFuncSHA256(t *testing.T) {
	tests := []struct {
		args []string
		want string
	}{
		{nil, ""},
		{[]string{"hello"}, "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"},
	}
	for _, tt := range tests {
		got := funcSHA256(tt.args).(string)
		if got != tt.want {
			t.Errorf("sha256(%v) = %q, want %q", tt.args, got, tt.want)
		}
	}
}

func TestFuncURLEncode(t *testing.T) {
	tests := []struct {
		args []string
		want string
	}{
		{nil, ""},
		{[]string{"hello world"}, "hello+world"},
		{[]string{"a=b&c=d"}, "a%3Db%26c%3Dd"},
	}
	for _, tt := range tests {
		got := funcURLEncode(tt.args).(string)
		if got != tt.want {
			t.Errorf("urlEncode(%v) = %q, want %q", tt.args, got, tt.want)
		}
	}
}

func TestFuncURLDecode(t *testing.T) {
	tests := []struct {
		args []string
		want string
	}{
		{nil, ""},
		{[]string{"hello+world"}, "hello world"},
		{[]string{"a%3Db%26c%3Dd"}, "a=b&c=d"},
		{[]string{"%ZZ"}, "%ZZ"}, // invalid encoding returns original
	}
	for _, tt := range tests {
		got := funcURLDecode(tt.args).(string)
		if got != tt.want {
			t.Errorf("urlDecode(%v) = %q, want %q", tt.args, got, tt.want)
		}
	}
}

func TestFuncDate(t *testing.T) {
	// Default format
	val := funcDate(nil).(string)
	_, err := time.Parse("2006-01-02", val)
	if err != nil {
		t.Errorf("date() = %q, not valid YYYY-MM-DD: %v", val, err)
	}

	// Custom format
	val = funcDate([]string{"2006/01/02"}).(string)
	_, err = time.Parse("2006/01/02", val)
	if err != nil {
		t.Errorf("date('2006/01/02') = %q, not valid: %v", val, err)
	}
}

func TestFuncJSON(t *testing.T) {
	if funcJSON(nil).(string) != "" {
		t.Error("json() with no args should return empty string")
	}
	if funcJSON([]string{`{"a":1}`}).(string) != `{"a":1}` {
		t.Error("json should pass through input")
	}
}

func TestFuncEnv(t *testing.T) {
	// No args
	if funcEnv(nil).(string) != "" {
		t.Error("env() with no args should return empty string")
	}

	// Set and read env var
	t.Setenv("HITSPEC_TEST_VAR", "test-value")
	got := funcEnv([]string{"HITSPEC_TEST_VAR"}).(string)
	if got != "test-value" {
		t.Errorf("env(HITSPEC_TEST_VAR) = %q, want %q", got, "test-value")
	}

	// Missing var with default
	got = funcEnv([]string{"HITSPEC_NONEXISTENT", "fallback"}).(string)
	if got != "fallback" {
		t.Errorf("env(NONEXISTENT, fallback) = %q, want %q", got, "fallback")
	}

	// Missing var without default
	got = funcEnv([]string{"HITSPEC_NONEXISTENT"}).(string)
	if got != "" {
		t.Errorf("env(NONEXISTENT) = %q, want empty", got)
	}
}

func TestParseArgs(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"a,b,c", []string{"a", "b", "c"}},
		{`"hello, world",foo`, []string{"hello, world", "foo"}},
		{`'single quoted'`, []string{"single quoted"}},
		{"  spaced , args ", []string{"spaced", "args"}},
		{"single", []string{"single"}},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseArgs(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("parseArgs(%q) = %v (len %d), want %v (len %d)", tt.input, got, len(got), tt.want, len(tt.want))
			}
			for i, g := range got {
				if g != tt.want[i] {
					t.Errorf("arg[%d] = %q, want %q", i, g, tt.want[i])
				}
			}
		})
	}
}
