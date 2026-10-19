package serve

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A stress run that stops before producing numbers (here a setup request that
// cannot connect) reports why, instead of a clean result with zero requests.
func TestStressResultCarriesRunError(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	closed := "http://" + l.Addr().String()
	_ = l.Close()

	dir := t.TempDir()
	src := "### login\n# @name login\n# @stress.setup\nPOST " + closed + "/login\n\n### load\n# @name load\nGET " + closed + "/x\n"
	if err := os.WriteFile(filepath.Join(dir, "a.http"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	s := NewServer(WithWorkDir(dir), WithLogLevel("error"))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.ctx, s.cancel = ctx, cancel

	result := startStressAndWait(t, s, `{"files":["a.http"],"duration":"1s","rate":5}`)
	if !strings.Contains(result.Error, `setup request "login" failed`) {
		t.Fatalf("result.Error = %q; want the setup failure", result.Error)
	}
	if result.Total != 0 {
		t.Fatalf("total = %d; no load should have run", result.Total)
	}
}
