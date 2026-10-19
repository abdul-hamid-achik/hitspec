package insomnia

import (
	"os"
	"strings"
	"testing"

	"github.com/abdul-hamid-achik/hitspec/v2/packages/import/internal/importtest"
)

func TestGolden(t *testing.T) {
	data, err := os.ReadFile("testdata/export.json")
	if err != nil {
		t.Fatal(err)
	}
	res, err := NewConverter().ConvertResult(data)
	if err != nil {
		t.Fatal(err)
	}
	f := importtest.Validate(t, res.Content)
	if len(f.Requests) != 5 {
		t.Errorf("want 5 requests, got %d", len(f.Requests))
	}
	y, err := res.EnvironmentsYAML()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(y, "port: 1234567\n") || strings.Contains(y, "e+") {
		t.Errorf("a 7-digit number must stay plain in the YAML:\n%s", y)
	}
	out := res.Content + "\n# environments:\n" + y + "\n# warnings:\n# - " + strings.Join(res.Warnings, "\n# - ") + "\n"
	importtest.Golden(t, "export.http", out)
}

func TestBaseEnvironmentOnlyBecomesVariables(t *testing.T) {
	export := `{"resources":[
		{"_id":"w","_type":"workspace"},
		{"_id":"e","_type":"environment","parentId":"w","name":"Base","data":{"baseUrl":"http://x","n":2}},
		{"_id":"r","_type":"request","parentId":"w","name":"A","method":"GET","url":"{{ _.baseUrl }}/a"}]}`
	res, err := NewConverter().ConvertResult([]byte(export))
	if err != nil {
		t.Fatal(err)
	}
	if res.HasEnvironments() {
		t.Error("a lone base environment must not produce hitspec.yaml environments")
	}
	for _, want := range []string{"@baseUrl = http://x", "@n = 2"} {
		if !strings.Contains(res.Content, want) {
			t.Errorf("missing %q in:\n%s", want, res.Content)
		}
	}
	importtest.Validate(t, res.Content)
}
