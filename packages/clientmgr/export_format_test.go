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
