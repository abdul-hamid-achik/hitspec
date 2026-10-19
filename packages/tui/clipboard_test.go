package tui

import (
	"context"
	"strings"
	"testing"

	"github.com/abdul-hamid-achik/hitspec/v2/packages/clientmgr"
)

const sampleHTTP = `### Ping
GET https://example.com/api/widgets
Accept: application/json
`

func TestExportTextCurl(t *testing.T) {
	mgr := newTestManager(t)
	ctx := context.Background()
	if _, err := mgr.CreateFile(ctx, "sample.http", sampleHTTP); err != nil {
		t.Fatalf("create file: %v", err)
	}
	out, err := exportText(ctx, mgr, clientmgr.ExportSelectionReq{File: "sample.http", Format: "curl"})
	if err != nil {
		t.Fatalf("exportText: %v", err)
	}
	if !strings.Contains(out, "curl") || !strings.Contains(out, "https://example.com/api/widgets") {
		t.Fatalf("curl export missing expected content: %q", out)
	}
}

func TestExportTextRequiresFile(t *testing.T) {
	mgr := newTestManager(t)
	if _, err := exportText(context.Background(), mgr, clientmgr.ExportSelectionReq{Format: "curl"}); err == nil {
		t.Fatal("expected an error when no file is provided")
	}
}

func TestExportCmdEmitsCopyMsg(t *testing.T) {
	mgr := newTestManager(t)
	ctx := context.Background()
	if _, err := mgr.CreateFile(ctx, "sample.http", sampleHTTP); err != nil {
		t.Fatalf("create file: %v", err)
	}
	msg := exportCmd(ctx, mgr, clientmgr.ExportSelectionReq{File: "sample.http", Format: "curl"})()
	cm, ok := msg.(copyMsg)
	if !ok {
		t.Fatalf("expected copyMsg, got %T", msg)
	}
	if cm.err != nil {
		t.Fatalf("copyMsg carried error: %v", cm.err)
	}
	if !strings.Contains(cm.content, "curl") {
		t.Fatalf("copyMsg content missing curl snippet: %q", cm.content)
	}
}

func TestExportCmdErrorsForMissingRequest(t *testing.T) {
	mgr := newTestManager(t)
	ctx := context.Background()
	if _, err := mgr.CreateFile(ctx, "sample.http", sampleHTTP); err != nil {
		t.Fatalf("create file: %v", err)
	}
	msg := exportCmd(ctx, mgr, clientmgr.ExportSelectionReq{File: "sample.http", RequestName: "does-not-exist", Format: "curl"})()
	cm := msg.(copyMsg)
	if cm.err == nil {
		t.Fatal("expected error for unknown request name")
	}
}

func TestCopyTextCmdEmpty(t *testing.T) {
	cm := copyTextCmd("", "body")().(copyMsg)
	if cm.err == nil {
		t.Fatal("expected error when copying empty text")
	}
}
