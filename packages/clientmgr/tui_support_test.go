package clientmgr

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/abdul-hamid-achik/hitspec/v2/internal/testhome"
)

func TestExecuteSourceRunsTheBuffer(t *testing.T) {
	srv := okServer(t)
	onDisk := "### one\n# @name one\nGET " + srv.URL + "\n\n>>>\nexpect status 200\n<<<\n"
	m := configWorkspace(t, "", onDisk)
	ctx := context.Background()
	edited := strings.Replace(onDisk, "200", "201", 1)

	res, err := m.ExecuteSource(ctx, ExecuteReq{File: "api.http", RequestName: "one"}, edited)
	if err != nil {
		t.Fatal(err)
	}
	if res.Failed != 1 {
		t.Fatalf("buffer asserts 201 against a 200 and must fail: %+v", res)
	}
	res, err = m.Execute(ctx, ExecuteReq{File: "api.http", RequestName: "one"})
	if err != nil || res.Passed != 1 {
		t.Fatalf("the file on disk still passes: err=%v res=%+v", err, res)
	}
}

func TestExecuteSourceSelectsByIndexOrName(t *testing.T) {
	srv := okServer(t)
	m := configWorkspace(t, "", "### x\nGET "+srv.URL+"\n")
	ctx := context.Background()
	buf := "### first\nGET " + srv.URL + "\n\n>>>\nexpect status 200\n<<<\n\n### second\nGET " + srv.URL + "\n\n>>>\nexpect status 500\n<<<\n"

	idx := 1
	res, err := m.ExecuteSource(ctx, ExecuteReq{File: "api.http", RequestIndex: &idx}, buf)
	if err != nil || len(res.Results) != 1 || res.Failed != 1 {
		t.Fatalf("untitled request 2 by index: err=%v res=%+v", err, res)
	}
	// A name missing from the buffer falls back to the index.
	res, err = m.ExecuteSource(ctx, ExecuteReq{File: "api.http", RequestName: "gone", RequestIndex: &idx}, buf)
	if err != nil || res.Failed != 1 {
		t.Fatalf("fallback to index: err=%v res=%+v", err, res)
	}
	out := 9
	if _, err := m.ExecuteSource(ctx, ExecuteReq{File: "api.http", RequestIndex: &out}, buf); err == nil {
		t.Fatal("an index past the end of the buffer should be an error")
	}
	if _, err := m.ExecuteSource(ctx, ExecuteReq{File: "api.http", RequestName: "one"}, "### x\nNOPE\n"); err == nil {
		t.Fatal("an unparseable buffer should be an error")
	}
}

func TestRunFileSourceAndParseSource(t *testing.T) {
	srv := okServer(t)
	m := configWorkspace(t, "", "### x\nGET "+srv.URL+"\n")
	ctx := context.Background()
	buf := "### a\nGET " + srv.URL + "\n\n### b\nGET " + srv.URL + "\n"

	parsed, err := m.ParseSource(ctx, "api.http", buf)
	if err != nil || len(parsed.Requests) != 2 {
		t.Fatalf("ParseSource: err=%v parsed=%+v", err, parsed)
	}
	if _, err := m.ParseSource(ctx, "api.http", "### x\nNOPE\n"); err == nil {
		t.Fatal("ParseSource should report a parse error")
	}
	res, err := m.RunFileSource(ctx, RunReq{File: "api.http"}, buf)
	if err != nil || len(res.Results) != 2 {
		t.Fatalf("RunFileSource: err=%v res=%+v", err, res)
	}
}

func TestExportSelectionUnnamedAndEnvironment(t *testing.T) {
	yaml := "defaultEnvironment: dev\nenvironments:\n  dev:\n    baseUrl: http://dev.example\n    token: dev-token\n  qa:\n    baseUrl: http://qa.example\n    token: qa-token\n"
	file := "@local = fileval\n\n### named\n# @name named\nGET {{baseUrl}}/named\n\n###\nGET {{baseUrl}}/untitled?x={{local}}\nAuthorization: Bearer {{token}}\n"
	m := configWorkspace(t, yaml, file)
	ctx := context.Background()

	idx := 1
	res, err := m.ExportSelection(ctx, ExportSelectionReq{File: "api.http", RequestIndex: &idx, Format: "curl"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Commands) != 1 {
		t.Fatalf("one request expected: %v", res.Commands)
	}
	cmd := res.Commands[0]
	for _, want := range []string{"http://dev.example/untitled?x=fileval", "Bearer dev-token"} {
		if !strings.Contains(cmd, want) {
			t.Fatalf("export should resolve variables (%q) from the active environment: %s", want, cmd)
		}
	}
	// Another environment resolves differently.
	res, err = m.ExportSelection(ctx, ExportSelectionReq{File: "api.http", RequestIndex: &idx, Format: "curl", Environment: "qa"})
	if err != nil || !strings.Contains(res.Commands[0], "http://qa.example/untitled") {
		t.Fatalf("qa export: err=%v %v", err, res)
	}
	// Named selection and the whole file.
	name := "named"
	res, err = m.ExportSelection(ctx, ExportSelectionReq{File: "api.http", RequestName: name, Format: "python"})
	if err != nil || len(res.Commands) != 1 || !strings.Contains(res.Commands[0], "http://dev.example/named") {
		t.Fatalf("named export: err=%v %v", err, res)
	}
	res, err = m.ExportSelection(ctx, ExportSelectionReq{File: "api.http", Format: "curl"})
	if err != nil || len(res.Commands) != 2 {
		t.Fatalf("whole-file export: err=%v %v", err, res)
	}
	// An unsaved buffer is exported instead of the file.
	res, err = m.ExportSelection(ctx, ExportSelectionReq{File: "api.http", Source: "### z\nGET http://buffer.example/z\n", RequestIndex: new(int), Format: "curl"})
	if err != nil || !strings.Contains(res.Commands[0], "buffer.example") {
		t.Fatalf("buffer export: err=%v %v", err, res)
	}
	bad := 7
	if _, err := m.ExportSelection(ctx, ExportSelectionReq{File: "api.http", RequestIndex: &bad, Format: "curl"}); err == nil {
		t.Fatal("an out-of-range request should be an error")
	}
	if _, err := m.ExportSelection(ctx, ExportSelectionReq{File: "api.http", Format: "nope"}); err == nil {
		t.Fatal("an unknown format should be an error")
	}
}

func TestCookieStorePathIsPerUserNotPerWorkspace(t *testing.T) {
	home := t.TempDir()
	testhome.Set(t, home)
	m := configWorkspace(t, "", "### x\nGET http://x\n")
	if got, want := m.CookieStorePath(), filepath.Join(home, ".hitspec", "tui-cookies.json"); got != want {
		t.Fatalf("CookieStorePath = %q, want %q", got, want)
	}
	if _, err := os.Stat(home); err != nil {
		t.Fatal(err)
	}
}
