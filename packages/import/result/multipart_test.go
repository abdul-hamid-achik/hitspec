package result

import (
	"testing"

	"github.com/abdul-hamid-achik/hitspec/v2/packages/core/parser"
)

// The line an importer writes for a multipart file must parse back to the same
// field name and path.
func TestMultipartFileLineRoundTrip(t *testing.T) {
	cases := []struct {
		name, path       string
		wantName, wantOK string
	}{
		{"avatar", "./me.png", "avatar", "named"},
		{"file", "./a.txt", "file", "named"},
		{"my doc", "./b.pdf", "file", "unnamed"}, // spaces: not expressible
		{"", "./c.bin", "file", "unnamed"},
		{"@odd", "./d.bin", "file", "unnamed"},
	}
	for _, c := range cases {
		line := MultipartFileLine(c.name, c.path)
		src := "### u\nPOST http://x.test/up\n\n>>>multipart\n" + line + "\n<<<\n"
		f, err := parser.Parse(src, "t.http")
		if err != nil {
			t.Fatalf("%q: %v", line, err)
		}
		fields := f.Requests[0].Body.Multipart
		if len(fields) != 1 {
			t.Fatalf("%q: %d fields", line, len(fields))
		}
		if fields[0].Name != c.wantName || fields[0].Path != c.path {
			t.Errorf("%q parsed to name %q path %q, want %q %q", line, fields[0].Name, fields[0].Path, c.wantName, c.path)
		}
		if got := CanNameMultipartFile(c.name); got != (c.wantOK == "named") {
			t.Errorf("CanNameMultipartFile(%q) = %v", c.name, got)
		}
	}
}
