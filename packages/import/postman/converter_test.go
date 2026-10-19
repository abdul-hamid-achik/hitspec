package postman

import (
	"os"
	"strings"
	"testing"

	"github.com/abdul-hamid-achik/hitspec/v2/packages/import/internal/importtest"
)

func TestGoldenCollection(t *testing.T) {
	data, err := os.ReadFile("testdata/collection.json")
	if err != nil {
		t.Fatal(err)
	}
	res, err := NewConverter().Convert(data)
	if err != nil {
		t.Fatal(err)
	}
	f := importtest.Validate(t, res.Content)
	if len(f.Requests) != 7 {
		t.Errorf("want 7 requests, got %d", len(f.Requests))
	}
	out := res.Content + "\n# warnings:\n# - " + strings.Join(res.Warnings, "\n# - ") + "\n"
	importtest.Golden(t, "collection.http", out)
}

func TestNoFakeAssertions(t *testing.T) {
	// A request with no test script must not get an invented `status == 200`.
	res, err := NewConverter().Convert([]byte(`{"info":{"name":"x"},"item":[{"name":"a","request":{"method":"GET","url":"http://h/x"}}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(res.Content, "expect") {
		t.Errorf("unexpected assertion:\n%s", res.Content)
	}
}

func TestEnvironmentExport(t *testing.T) {
	data, err := os.ReadFile("testdata/environment.json")
	if err != nil {
		t.Fatal(err)
	}
	res, err := NewConverter().Convert(data)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(res.Content) != "" {
		t.Errorf("an environment export has no requests, got content %q", res.Content)
	}
	y, err := res.EnvironmentsYAML()
	if err != nil {
		t.Fatal(err)
	}
	importtest.Golden(t, "environment.yaml", y)
}

func TestTranslateScript(t *testing.T) {
	cases := []struct {
		src  string
		want []string
		todo int
	}{
		{`pm.response.to.have.status(201);`, []string{"status == 201"}, 0},
		{`pm.expect(pm.response.code).to.eql(404)`, []string{"status == 404"}, 0},
		{`pm.response.to.be.ok;`, []string{"status == 200"}, 0},
		{`const d = pm.response.json(); pm.expect(d.a.b).to.equal("x");`, []string{`body.a.b == "x"`}, 0},
		{`pm.expect(jsonData.n).to.not.eql(3)`, []string{`body.n != 3`}, 0},
		{`pm.expect(pm.response.headers.get("Content-Type")).to.include("json")`, []string{`header Content-Type contains "json"`}, 0},
		{`pm.expect(jsonData.items).to.have.lengthOf(3)`, []string{`body.items length 3`}, 0},
		{`pm.expect(jsonData.id).to.be.oneOf([1, 2])`, []string{`body.id in [1, 2]`}, 0},
		{`tests["ok"] = responseCode.code === 200;`, []string{"status == 200"}, 0},
		{"pm.test('x', function () {\n  pm.expect(foo()).to.be.true;\n});", nil, 1},
		{`pm.environment.set("t", jsonData.token);`, nil, 1},
		{`// pm.response.to.have.status(500)`, nil, 0},
	}
	for _, tc := range cases {
		got := translateScript(tc.src)
		if strings.Join(got.Assertions, "|") != strings.Join(tc.want, "|") {
			t.Errorf("%q: assertions = %q, want %q", tc.src, got.Assertions, tc.want)
		}
		if len(got.Untranslated) != tc.todo {
			t.Errorf("%q: untranslated = %q, want %d", tc.src, got.Untranslated, tc.todo)
		}
	}
}
