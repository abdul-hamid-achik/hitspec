package parser

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// update regenerates the golden snapshots: go test ./packages/core/parser -run TestGolden -update
var update = flag.Bool("update", false, "rewrite the parser golden files")

const (
	goldenExamplesDir = "../../../examples"
	goldenDir         = "testdata/golden"
)

// enumNames renders the integer enums of the AST by name so a golden diff reads
// as "operator: == -> contains" rather than "operator: 0 -> 6".
var enumNames = map[reflect.Type]func(int) string{
	reflect.TypeOf(AssertionOperator(0)): func(i int) string { return AssertionOperator(i).String() },
	reflect.TypeOf(CaptureSource(0)):     func(i int) string { return CaptureSource(i).String() },
	reflect.TypeOf(AuthType(0)): func(i int) string {
		names := []string{"none", "basic", "bearer", "apikey", "apikey-query", "digest", "aws", "oauth2-client-credentials", "oauth2-password"}
		if i >= 0 && i < len(names) {
			return names[i]
		}
		return fmt.Sprintf("AuthType(%d)", i)
	},
	reflect.TypeOf(BodyType(0)): func(i int) string {
		names := []string{"none", "json", "form", "form-block", "multipart", "raw", "xml", "graphql", "file"}
		if i >= 0 && i < len(names) {
			return names[i]
		}
		return fmt.Sprintf("BodyType(%d)", i)
	},
	reflect.TypeOf(ConditionType(0)): func(i int) string {
		if i == int(ConditionUnless) {
			return "unless"
		}
		return "if"
	},
	reflect.TypeOf(HookType(0)): func(i int) string {
		names := []string{"set", "log", "assert", "exec"}
		if i >= 0 && i < len(names) {
			return names[i]
		}
		return fmt.Sprintf("HookType(%d)", i)
	},
	reflect.TypeOf(MultipartFieldType(0)): func(i int) string {
		if i == int(MultipartFieldFile) {
			return "file"
		}
		return "value"
	},
}

// snapshotValue converts any AST value into a JSON-friendly tree. Struct fields
// are walked by reflection so a field added to the AST shows up in the goldens
// without touching this test.
func snapshotValue(v reflect.Value) any {
	if !v.IsValid() {
		return nil
	}
	if name, ok := enumNames[v.Type()]; ok {
		return name(int(v.Int()))
	}
	switch v.Kind() {
	case reflect.Pointer, reflect.Interface:
		if v.IsNil() {
			return nil
		}
		return snapshotValue(v.Elem())
	case reflect.Struct:
		out := map[string]any{}
		for i := 0; i < v.NumField(); i++ {
			if !v.Type().Field(i).IsExported() {
				continue
			}
			// File.Warnings only appears in a golden when a file produces one,
			// so adding the field did not touch every snapshot.
			// Body.FormFields is the same: only form-block bodies have any.
			if name := v.Type().Field(i).Name; (name == "Warnings" || name == "FormFields") && v.Field(i).Len() == 0 {
				continue
			}
			out[v.Type().Field(i).Name] = snapshotValue(v.Field(i))
		}
		return out
	case reflect.Slice:
		if v.IsNil() {
			return nil
		}
		out := make([]any, v.Len())
		for i := range out {
			out[i] = snapshotValue(v.Index(i))
		}
		return out
	case reflect.Map:
		if v.IsNil() {
			return nil
		}
		out := map[string]any{}
		for _, k := range v.MapKeys() {
			out[fmt.Sprint(k.Interface())] = snapshotValue(v.MapIndex(k))
		}
		return out
	case reflect.String:
		return v.String()
	case reflect.Bool:
		return v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int()
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return v.Uint()
	case reflect.Float32, reflect.Float64:
		return v.Float()
	default:
		return fmt.Sprint(v.Interface())
	}
}

// TestGolden parses every example file and compares the full parse result
// (requests, headers in order, bodies, metadata incl. auth, assertions,
// captures, file variables) against a committed JSON snapshot. A parser change
// that silently alters how an example is understood fails here and shows up as
// a reviewable diff.
func TestGolden(t *testing.T) {
	var inputs []string
	err := filepath.WalkDir(goldenExamplesDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && (strings.HasSuffix(path, ".http") || strings.HasSuffix(path, ".hitspec")) {
			inputs = append(inputs, path)
		}
		return nil
	})
	require.NoError(t, err)
	require.NotEmpty(t, inputs, "no example files found")

	if *update {
		require.NoError(t, os.MkdirAll(goldenDir, 0o755))
	}

	for _, input := range inputs {
		rel, err := filepath.Rel(goldenExamplesDir, input)
		require.NoError(t, err)
		name := strings.ReplaceAll(filepath.ToSlash(rel), "/", "__")
		t.Run(name, func(t *testing.T) {
			file, perr := ParseFile(input)
			snap := map[string]any{}
			if perr != nil {
				snap["ParseError"] = perr.Error()
			} else {
				file.Path = filepath.ToSlash(rel)
				snap["File"] = snapshotValue(reflect.ValueOf(file))
			}
			got, err := json.MarshalIndent(snap, "", "  ")
			require.NoError(t, err)
			got = append(got, '\n')

			golden := filepath.Join(goldenDir, name+".json")
			if *update {
				require.NoError(t, os.WriteFile(golden, got, 0o644))
				return
			}
			want, err := os.ReadFile(golden)
			require.NoError(t, err, "missing golden; run with -update")
			require.Equal(t, strings.Split(string(want), "\n"), strings.Split(string(got), "\n"), "parse of %s changed; review the diff and run with -update if intended", input)
		})
	}
}
