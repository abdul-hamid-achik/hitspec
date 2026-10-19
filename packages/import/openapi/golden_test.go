package openapi

import (
	"strings"
	"testing"

	"github.com/abdul-hamid-achik/hitspec/v2/packages/import/internal/importtest"
)

func TestGoldenPetstore(t *testing.T) {
	res, err := NewConverter().ConvertFileResult("testdata/petstore.yaml")
	if err != nil {
		t.Fatal(err)
	}
	f := importtest.Validate(t, res.Content)
	if len(f.Requests) != 4 {
		t.Errorf("want 4 requests, got %d", len(f.Requests))
	}
	// Path params must be backed by variables, not left unresolved.
	for _, want := range []string{"@id = 123e4567-e89b-12d3-a456-426614174000", "@getUser_id = 1", "@petId = 42", "GET {{baseUrl}}/users/{{getUser_id}}"} {
		if !strings.Contains(res.Content, want) {
			t.Errorf("missing %q", want)
		}
	}
	out := res.Content + "\n# warnings:\n# - " + strings.Join(res.Warnings, "\n# - ") + "\n"
	importtest.Golden(t, "petstore.http", out)
}

func TestConvertDeterministic(t *testing.T) {
	first, err := NewConverter().ConvertFileResult("testdata/petstore.yaml")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 20; i++ {
		again, err := NewConverter().ConvertFileResult("testdata/petstore.yaml")
		if err != nil {
			t.Fatal(err)
		}
		if again.Content != first.Content {
			t.Fatal("output differs between runs")
		}
	}
}

func TestConvertFilesSplitsByTag(t *testing.T) {
	split, err := NewConverter().ConvertFilesResult("testdata/petstore.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, f := range split.Files {
		names = append(names, f.Name)
		got := importtest.Validate(t, f.Content)
		switch f.Name {
		case "pets.http":
			if len(got.Requests) != 2 {
				t.Errorf("pets.http: want 2 requests, got %d", len(got.Requests))
			}
			if strings.Contains(f.Content, "@getUser_id") {
				t.Errorf("pets.http must not define the users-only variable:\n%s", f.Content)
			}
		case "users.http":
			if !strings.Contains(f.Content, "@getUser_id = 1") {
				t.Errorf("users.http must define its path variable:\n%s", f.Content)
			}
		}
		if !strings.Contains(f.Content, "@baseUrl = https://pets.example.com/v1") {
			t.Errorf("%s lacks @baseUrl", f.Name)
		}
	}
	if strings.Join(names, ",") != "health.http,pets.http,users.http" {
		t.Errorf("files = %v", names)
	}
}
