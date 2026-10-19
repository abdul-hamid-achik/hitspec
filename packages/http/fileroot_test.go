package http

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/abdul-hamid-achik/hitspec/v2/packages/core/parser"
)

func bodyFileReq(path string) *parser.Request {
	return &parser.Request{Body: &parser.Body{ContentType: parser.BodyFile, FilePath: path}}
}

func TestCheckFileRefs(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{filepath.Join(root, "body.json"), filepath.Join(outside, "secret.txt")} {
		if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(outside, filepath.Join(root, "link")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	id := func(s string) string { return s }
	base := filepath.Join(root, "sub")

	for _, tc := range []struct {
		name, ref string
		ok        bool
	}{
		{"relative inside", "../body.json", true},
		{"missing inside", "nope.json", true},
		{"absolute inside", filepath.Join(root, "body.json"), true},
		{"absolute outside", filepath.Join(outside, "secret.txt"), false},
		{"traversal", "../../etc/hosts", false},
		{"symlink escape", "../link/secret.txt", false},
		{"missing under symlink escape", "../link/new.txt", false},
	} {
		err := CheckFileRefs(bodyFileReq(tc.ref), id, base, root)
		if (err == nil) != tc.ok {
			t.Errorf("%s (%s): err = %v, want ok=%v", tc.name, tc.ref, err, tc.ok)
		}
	}

	// Multipart file parts are confined too; plain fields are not files.
	mp := &parser.Request{Body: &parser.Body{ContentType: parser.BodyMultipart, Multipart: []*parser.MultipartField{
		{Type: parser.MultipartFieldValue, Name: "t", Value: "/etc/hosts"},
		{Type: parser.MultipartFieldFile, Name: "f", Path: filepath.Join(outside, "secret.txt")},
	}}}
	if err := CheckFileRefs(mp, id, base, root); err == nil {
		t.Error("multipart file outside the root accepted")
	}

	// Variables are resolved before the check.
	if err := CheckFileRefs(bodyFileReq("{{f}}"), func(string) string { return "/etc/hosts" }, base, root); err == nil {
		t.Error("variable resolving to an outside path accepted")
	}

	// No root: nothing is enforced (the CLI keeps its behavior).
	if err := CheckFileRefs(bodyFileReq("/etc/hosts"), id, base, ""); err != nil {
		t.Errorf("empty root should disable the check: %v", err)
	}
}
