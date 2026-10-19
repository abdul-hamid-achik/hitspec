package snippet

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/abdul-hamid-achik/hitspec/v2/packages/core/parser"
	hitspechttp "github.com/abdul-hamid-achik/hitspec/v2/packages/http"
	"strings"
)

// A "& key = value" form block keeps its values unencoded in FormFields. Every
// export format must print the bytes the client sends: the encoded form.
func TestFormBlockExportsEncodedBody(t *testing.T) {
	src := "### Login\n# @name login\nPOST https://api.example.com/login\n\n& name = Zoë Smith\n& note = a & b=c\n"
	path := filepath.Join(t.TempDir(), "f.http")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := parser.ParseFile(path)
	if err != nil {
		t.Fatal(err)
	}
	req := f.Requests[0]
	if req.Body == nil || req.Body.ContentType != parser.BodyFormBlock {
		t.Fatalf("not a form block: %+v", req.Body)
	}

	// What hitspec actually sends.
	sent := string(hitspechttp.BuildRequestFromAST(req, func(s string) string { return s }).Body)
	const want = "name=Zo%C3%AB%20Smith&note=a%20%26%20b%3Dc"
	if sent != want {
		t.Fatalf("client body = %q, want %q", sent, want)
	}

	for _, format := range Formats {
		out, err := Render(format, req)
		if err != nil {
			t.Fatalf("%s: %v", format, err)
		}
		if !strings.Contains(out, sent) {
			t.Errorf("%s: output lacks the encoded body %q:\n%s", format, sent, out)
		}
		if strings.Contains(out, "Zoë") || strings.Contains(out, "a & b") {
			t.Errorf("%s: output carries the unencoded value:\n%s", format, out)
		}
	}
}
