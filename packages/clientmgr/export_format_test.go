package clientmgr

import (
	"context"
	"strings"
	"testing"
)

// The Studio export action shares its renderers with `hitspec export`; an
// unknown format used to come back as a bare "METHOD URL" line.
func TestExportUnknownFormatErrors(t *testing.T) {
	m := newTestManager(t)
	writeRunnableFile(t, m, "api.http", "http://127.0.0.1:1")

	_, err := m.Export(context.Background(), ExportReq{File: "api.http", RequestName: "ping", Format: "cobol"})
	if err == nil || !strings.Contains(err.Error(), "unknown export format") {
		t.Fatalf("want unknown export format error, got %v", err)
	}
	for _, format := range []string{"curl", "fetch", "wget", "python", "httpie", "go", "ruby", "javascript"} {
		out, err := m.Export(context.Background(), ExportReq{File: "api.http", RequestName: "ping", Format: format})
		if err != nil || len(out.Commands) != 1 || out.Commands[0] == "" {
			t.Errorf("Export(%s) = %v, %v", format, out, err)
		}
	}
}

// Exporting a whole file (empty RequestName, what the TUI does) must survive a
// request the format cannot express: it is replaced by a SKIPPED comment, and
// only a file where nothing can be exported is an error.
func TestExportWholeFileSkipsInexpressibleRequests(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	content := "### ok\n# @name ok\nGET http://127.0.0.1:1/ok\n\n" +
		"### up\n# @name up\nPOST http://127.0.0.1:1/up\n\n>>>multipart\nfield a = b\n<<<\n"
	if _, err := m.CreateFile(ctx, "mix.http", content); err != nil {
		t.Fatal(err)
	}

	out, err := m.Export(ctx, ExportReq{File: "mix.http", Format: "python"})
	if err != nil {
		t.Fatalf("whole-file export must not fail on one multipart request: %v", err)
	}
	if len(out.Commands) != 2 || !strings.Contains(out.Commands[0], "import requests") ||
		!strings.HasPrefix(out.Commands[1], "# SKIPPED: ") || !strings.Contains(out.Commands[1], "multipart") {
		t.Errorf("commands = %q", out.Commands)
	}

	// Asking for the inexpressible request by name is an error.
	if _, err := m.Export(ctx, ExportReq{File: "mix.http", RequestName: "up", Format: "python"}); err == nil {
		t.Error("exporting only the multipart request as python must error")
	}
	// curl handles multipart.
	if out, err := m.Export(ctx, ExportReq{File: "mix.http", Format: "curl"}); err != nil || len(out.Commands) != 2 || strings.Contains(out.Commands[1], "SKIPPED") {
		t.Errorf("curl export = %v, %v", out, err)
	}
}
