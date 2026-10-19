package curl

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/abdul-hamid-achik/hitspec/v2/packages/import/internal/importtest"
)

// goldenCases are curl commands whose conversion is snapshotted under
// testdata/ and must also pass `hitspec validate`.
var goldenCases = []struct{ name, cmd string }{
	{"multi_data", `curl -X POST https://api.example.com/x -d a=1 -d b=2 --data c=3`},
	{"data_file", `curl https://api.example.com/x -H 'Content-Type: application/json' -d @payload.json`},
	{"json_flag", `curl https://api.example.com/x --json '{"a":1}'`},
	{"form_multipart", `curl https://api.example.com/upload -F name=Jo -F 'avatar=@./me.png;type=image/png'`},
	{"data_urlencode", `curl https://api.example.com/x --data-urlencode 'q=a b&c' --data-urlencode '=plain text'`},
	{"basic_auth", `curl -u admin:s3cret https://api.example.com/admin`},
	{"basic_auth_comma", `curl -u 'admin:pa,ss' https://api.example.com/admin`},
	{"get_mode", `curl -G https://api.example.com/search -d q=go -d page=2`},
	{"unsupported", `curl -sSL --compressed -x http://proxy:8080 --retry 3 --weird-flag https://api.example.com/x -m 5`},
	{"chrome_copy", "curl 'https://api.example.com/x' \\\n  -H 'accept: */*' \\\n  --data-raw $'{\"a\":\"it\\'s\"}'"},
	{"header_order", `curl https://api.example.com/x -H 'Z-Last: 1' -H 'A-First: 2' -H 'M-Mid: 3'`},
	{"head_cookie", `curl -I -b 'sid=abc' https://api.example.com/x`},
}

func TestGolden(t *testing.T) {
	c := NewConverter()
	for _, tc := range goldenCases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := c.ConvertCommandResult(tc.cmd)
			if err != nil {
				t.Fatal(err)
			}
			importtest.Validate(t, res.Content)
			out := res.Content
			if len(res.Warnings) > 0 {
				out += "\n# warnings:\n# - " + strings.Join(res.Warnings, "\n# - ") + "\n"
			}
			importtest.Golden(t, tc.name+".http", out)
		})
	}
}

func TestConvertFileDedupesNamesAndValidates(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cmds.txt")
	body := "curl https://api.example.com/users\ncurl https://api.example.com/users\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := NewConverter().ConvertFileResult(path)
	if err != nil {
		t.Fatal(err)
	}
	f := importtest.Validate(t, res.Content)
	if len(f.Requests) != 2 || f.Requests[0].Name == f.Requests[1].Name {
		t.Fatalf("want two distinct request names, got %+v", f.Requests)
	}
}

func TestParse_Semantics(t *testing.T) {
	c := NewConverter()
	p, err := c.Parse(`curl -X GET https://x.test/a -d a=1`)
	if err != nil {
		t.Fatal(err)
	}
	if p.Method != "GET" {
		t.Errorf("explicit -X GET must survive -d, got %s", p.Method)
	}
	p, err = c.Parse(`curl -d a=1 -d b=2 https://x.test/a`)
	if err != nil {
		t.Fatal(err)
	}
	if p.Body != "a=1&b=2" {
		t.Errorf("multiple -d must join with &, got %q", p.Body)
	}
	p, err = c.Parse(`curl -sSLk https://x.test/a`)
	if err != nil {
		t.Fatal(err)
	}
	if p.URL != "https://x.test/a" || !p.Insecure || !p.FollowRedirects {
		t.Errorf("clustered short flags mis-parsed: %+v", p)
	}
	if len(p.Warnings) == 0 {
		t.Error("-k must produce a warning")
	}
	if _, err := c.Parse(`curl -XPOST`); err == nil {
		t.Error("missing URL must error")
	}
}
