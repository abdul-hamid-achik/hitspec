package clientmgr

import (
	"context"
	"strings"
	"testing"
)

// A cancelled context stops fetching a remote spec: the error is the
// cancellation, not a DNS failure from a fetch that went ahead anyway.
func TestImportOpenAPIHonorsContext(t *testing.T) {
	m := newTestManager(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := m.ImportOpenAPI(ctx, ImportOpenAPIReq{SpecPath: "https://spec.example.invalid/openapi.yaml"})
	if err == nil || !strings.Contains(err.Error(), "context canceled") {
		t.Fatalf("err = %v; want a context cancellation", err)
	}
}
