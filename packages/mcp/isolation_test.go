package mcp

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

const isolationSecret = "tvly-super-secret-value"

type recordedRequest struct {
	mu   sync.Mutex
	dump []string
}

func (r *recordedRequest) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		body, _ := io.ReadAll(req.Body)
		var headers []string
		for key, values := range req.Header {
			headers = append(headers, key+": "+strings.Join(values, ","))
		}
		r.mu.Lock()
		r.dump = append(r.dump, req.URL.String()+"\n"+strings.Join(headers, "\n")+"\n"+string(body))
		r.mu.Unlock()
		_, _ = io.WriteString(w, "ok")
	})
}

func (r *recordedRequest) all() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return strings.Join(r.dump, "\n---\n")
}

func writeWorkspaceFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

// leakingWorkspace builds a workspace, as a cloned repository might ship, whose
// saved request tries every route to the server's process environment.
func leakingWorkspace(t *testing.T, baseURL string) string {
	t.Helper()
	workspace := t.TempDir()
	writeWorkspaceFile(t, workspace, "hitspec.yaml",
		"environments:\n  dev:\n    cfgKey: \"${HITSPEC_TEST_SECRET}\"\n")
	writeWorkspaceFile(t, workspace, "leak.http", strings.Join([]string{
		"@viaVar = {{$env(HITSPEC_TEST_SECRET)}}",
		"",
		"### Leak",
		"# @name leak",
		"POST " + baseURL + "/x?a={{$env(HITSPEC_TEST_SECRET)}}&b={{HITSPEC_TEST_SECRET}}&c={{$HITSPEC_TEST_SECRET}}&d={{cfgKey}}&e={{viaVar}}",
		"X-Token: {{$env(HITSPEC_TEST_SECRET)}}",
		"Content-Type: application/json",
		"",
		`{"k":"{{$env(HITSPEC_TEST_SECRET)}}","j":"{{ $env( HITSPEC_TEST_SECRET ) }}"}`,
		"",
	}, "\n"))
	return workspace
}

func TestFetchSavedRequestCannotReadProcessEnvironment(t *testing.T) {
	t.Setenv("HITSPEC_TEST_SECRET", isolationSecret)
	recorder := &recordedRequest{}
	web := httptest.NewServer(recorder.handler())
	defer web.Close()
	workspace := leakingWorkspace(t, web.URL)
	server, err := NewServer("test", workspace, Options{AllowPrivateNetwork: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := server.handleFetch(context.Background(), nil, fetchInput{File: "leak.http", Name: "leak"}); err != nil {
		t.Fatal(err)
	}
	got := recorder.all()
	if got == "" {
		t.Fatal("fake server received no request")
	}
	if strings.Contains(got, isolationSecret) {
		t.Fatalf("process environment secret reached the network:\n%s", got)
	}
}

func TestFetchSavedRequestHonorsOperatorAllowlist(t *testing.T) {
	t.Setenv("HITSPEC_TEST_SECRET", isolationSecret)
	t.Setenv("HITSPEC_TEST_OTHER", "other-secret")
	recorder := &recordedRequest{}
	web := httptest.NewServer(recorder.handler())
	defer web.Close()
	workspace := t.TempDir()
	writeWorkspaceFile(t, workspace, "a.http",
		"POST "+web.URL+"/x?a={{$env(HITSPEC_TEST_SECRET)}}&b={{$env(HITSPEC_TEST_OTHER)}}\n")
	server, err := NewServer("test", workspace, Options{AllowPrivateNetwork: true, AllowedEnv: []string{"HITSPEC_TEST_SECRET"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := server.handleFetch(context.Background(), nil, fetchInput{File: "a.http"}); err != nil {
		t.Fatal(err)
	}
	got := recorder.all()
	if !strings.Contains(got, "a="+isolationSecret) || strings.Contains(got, "other-secret") {
		t.Fatalf("allowlist not honored:\n%s", got)
	}
}

func TestFetchSavedRequestUsesDotEnvWithoutExportingIt(t *testing.T) {
	_ = os.Unsetenv("HITSPEC_TEST_DOTENV")
	recorder := &recordedRequest{}
	web := httptest.NewServer(recorder.handler())
	defer web.Close()
	workspace := t.TempDir()
	writeWorkspaceFile(t, workspace, ".env", "HITSPEC_TEST_DOTENV=from-dotenv\n")
	writeWorkspaceFile(t, workspace, "a.http", "GET "+web.URL+"/x?v={{HITSPEC_TEST_DOTENV}}&w={{$env(HITSPEC_TEST_DOTENV)}}\n")
	writeWorkspaceFile(t, workspace, "b.http", "GET "+web.URL+"/y?v={{HITSPEC_TEST_DOTENV}}\n")
	server, err := NewServer("test", workspace, Options{AllowPrivateNetwork: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := server.handleFetch(context.Background(), nil, fetchInput{File: "a.http", EnvFile: ".env"}); err != nil {
		t.Fatal(err)
	}
	if got := recorder.all(); !strings.Contains(got, "v=from-dotenv") || !strings.Contains(got, "w=from-dotenv") {
		t.Fatalf(".env values were not applied:\n%s", got)
	}
	if v, ok := os.LookupEnv("HITSPEC_TEST_DOTENV"); ok {
		t.Fatalf(".env leaked into the process environment: %q", v)
	}
	// A later call without env_file must not see the earlier .env.
	recorder.dump = nil
	if _, _, err := server.handleFetch(context.Background(), nil, fetchInput{File: "b.http"}); err != nil {
		t.Fatal(err)
	}
	if got := recorder.all(); strings.Contains(got, "from-dotenv") {
		t.Fatalf(".env persisted across tool calls:\n%s", got)
	}
}

func TestFetchAnnotationsAreTruthful(t *testing.T) {
	server, err := NewServer("test", t.TempDir(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	session := connect(t, server)
	listed, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	seen := false
	for _, tool := range listed.Tools {
		switch tool.Name {
		case "hitspec_fetch":
			seen = true
			a := tool.Annotations
			if a == nil || a.ReadOnlyHint || a.DestructiveHint == nil || !*a.DestructiveHint ||
				a.OpenWorldHint == nil || !*a.OpenWorldHint {
				t.Errorf("hitspec_fetch annotations = %#v, want readOnly=false destructive=true openWorld=true", a)
			}
		case "hitspec_list_requests", "hitspec_validate":
			if a := tool.Annotations; a == nil || !a.ReadOnlyHint {
				t.Errorf("%s must stay read-only: %#v", tool.Name, a)
			}
		}
	}
	if !seen {
		t.Fatal("hitspec_fetch missing")
	}
	_ = sdkmcp.ToolAnnotations{}
}

func TestWorkspaceRefusesHomeAncestorsAndRoot(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	project := filepath.Join(home, "project")
	if err := os.Mkdir(project, 0o700); err != nil {
		t.Fatal(err)
	}
	for name, workspace := range map[string]string{
		"home":   home,
		"parent": filepath.Dir(home),
		"root":   string(filepath.Separator),
	} {
		if _, err := NewServer("test", workspace, Options{}); err == nil || !strings.Contains(err.Error(), "allow-broad-workspace") {
			t.Errorf("%s workspace %q: err = %v, want broad-workspace refusal", name, workspace, err)
		}
		if _, err := NewServer("test", workspace, Options{AllowBroadWorkspace: true}); err != nil {
			t.Errorf("%s workspace %q with explicit allow: %v", name, workspace, err)
		}
	}
	if _, err := NewServer("test", project, Options{}); err != nil {
		t.Errorf("project below home must be accepted: %v", err)
	}
	// "." resolved from the home directory is refused too.
	t.Chdir(home)
	if _, err := NewServer("test", ".", Options{}); err == nil {
		t.Error("default workspace from $HOME was accepted")
	}
}

func TestToolErrorsUseRelativePathsAndHideExistence(t *testing.T) {
	parent := t.TempDir()
	workspace := filepath.Join(parent, "workspace")
	if err := os.Mkdir(workspace, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(parent, "outside.http"), []byte("GET https://example.com\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(parent, "outside.http"), filepath.Join(workspace, "link.http")); err != nil {
		t.Fatal(err)
	}
	writeWorkspaceFile(t, workspace, "bad.http", "GET https://example.com\n\n>>>mock\n{}\n")
	writeWorkspaceFile(t, workspace, "two.http", "### A\n# @name a\nGET https://example.com\n\n### B\n# @name b\nGET https://example.com\n")
	server, err := NewServer("test", workspace, Options{})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	call := func(name string, run func() error) string {
		t.Helper()
		err := run()
		if err == nil {
			t.Fatalf("%s: expected an error", name)
		}
		if strings.Contains(err.Error(), parent) || strings.Contains(err.Error(), "/private") {
			t.Errorf("%s: error leaks an absolute path: %v", name, err)
		}
		return err.Error()
	}
	missing := call("validate missing", func() error {
		_, _, err := server.handleValidate(ctx, nil, validateInput{File: "nope.http"})
		return err
	})
	escaped := call("validate ../", func() error {
		_, _, err := server.handleValidate(ctx, nil, validateInput{File: "../outside.http"})
		return err
	})
	symlinked := call("validate symlink", func() error {
		_, _, err := server.handleValidate(ctx, nil, validateInput{File: "link.http"})
		return err
	})
	missingDeep := call("validate ../missing", func() error {
		_, _, err := server.handleValidate(ctx, nil, validateInput{File: "../does-not-exist.http"})
		return err
	})
	if missing != "path not found or not allowed" || escaped != missing || symlinked != missing || missingDeep != missing {
		t.Errorf("messages differ: missing=%q escaped=%q symlink=%q missingDeep=%q", missing, escaped, symlinked, missingDeep)
	}
	for name, input := range map[string]fetchInput{
		"fetch missing":    {File: "nope.http"},
		"fetch escape":     {File: "../outside.http"},
		"fetch env_file":   {File: "two.http", Name: "a", EnvFile: "../.env"},
		"fetch config":     {File: "two.http", Name: "a", ConfigFile: "../hitspec.yaml"},
		"fetch ambiguous":  {File: "two.http"},
		"fetch unknown":    {File: "two.http", Name: "zzz"},
		"fetch index high": {File: "two.http", Index: 9},
	} {
		err := call(name, func() error {
			_, _, err := server.handleFetch(ctx, nil, input)
			return err
		})
		if strings.Contains(name, "missing") || strings.Contains(name, "escape") || strings.Contains(name, "env_file") || strings.Contains(name, "config") {
			if err != missing {
				t.Errorf("%s: error = %q, want %q", name, err, missing)
			}
		}
		if name == "fetch ambiguous" && !strings.Contains(err, "two.http contains 2 requests") {
			t.Errorf("%s: want a workspace-relative file name, got %q", name, err)
		}
	}
	call("list missing", func() error {
		_, _, err := server.handleList(ctx, nil, listInput{Path: "nope"})
		return err
	})
	if msg := call("list parse error", func() error {
		_, _, err := server.handleList(ctx, nil, listInput{})
		return err
	}); !strings.Contains(msg, "bad.http") {
		t.Errorf("list error should name the relative file: %q", msg)
	}
	// validate reports parse errors in its payload; they must be relative too.
	result, _, err := server.handleValidate(ctx, nil, validateInput{File: "bad.http"})
	if err != nil {
		t.Fatal(err)
	}
	if text := result.Content[0].(*sdkmcp.TextContent).Text; strings.Contains(text, parent) {
		t.Errorf("validate payload leaks an absolute path: %s", text)
	}
}

func TestBodyFileErrorsAreUniform(t *testing.T) {
	parent := t.TempDir()
	workspace := filepath.Join(parent, "workspace")
	if err := os.Mkdir(workspace, 0o700); err != nil {
		t.Fatal(err)
	}
	writeWorkspaceFile(t, parent, "secret.txt", "secret")
	writeWorkspaceFile(t, workspace, "escape.http", "POST https://example.com\n\n< ../secret.txt\n")
	writeWorkspaceFile(t, workspace, "absent.http", "POST https://example.com\n\n< ../absent.txt\n")
	server, err := NewServer("test", workspace, Options{})
	if err != nil {
		t.Fatal(err)
	}
	var messages []string
	for _, file := range []string{"escape.http", "absent.http"} {
		_, _, err := server.handleFetch(context.Background(), nil, fetchInput{File: file})
		if err == nil {
			t.Fatalf("%s: expected an error", file)
		}
		messages = append(messages, err.Error())
	}
	if messages[0] != messages[1] || !strings.Contains(messages[0], "not found or not allowed") || strings.Contains(messages[0], parent) {
		t.Errorf("body file errors = %q", messages)
	}
}
