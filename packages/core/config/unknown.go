package config

import (
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// unknownKeyWarnings reports every mapping key in data that the Config struct
// does not define, as "<name>:<line>: unknown key ...". The known keys come
// from the yaml tags of Config (and the types it contains), so a field added
// to the struct is accepted without touching this file. Free-form sections
// (environments, headers) are not checked.
func unknownKeyWarnings(name string, data []byte) []string {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil || doc.Kind == 0 {
		return nil
	}
	var warnings []string
	walkKnownKeys(&doc, reflect.TypeOf(Config{}), func(key *yaml.Node, known []string) {
		msg := fmt.Sprintf("%s:%d: unknown key %q", name, key.Line, key.Value)
		if s := suggest(key.Value, known); s != "" {
			msg += fmt.Sprintf(" (did you mean %q?)", s)
		}
		warnings = append(warnings, msg)
	})
	return warnings
}

func walkKnownKeys(n *yaml.Node, t reflect.Type, report func(key *yaml.Node, known []string)) {
	for n.Kind == yaml.DocumentNode && len(n.Content) > 0 {
		n = n.Content[0]
	}
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	switch t.Kind() {
	case reflect.Struct:
		if n.Kind != yaml.MappingNode {
			return
		}
		fields := yamlFields(t)
		known := make([]string, 0, len(fields))
		for k := range fields {
			known = append(known, k)
		}
		sort.Strings(known)
		for i := 0; i+1 < len(n.Content); i += 2 {
			key, val := n.Content[i], n.Content[i+1]
			if key.Value == "<<" { // YAML merge key
				continue
			}
			ft, ok := fields[key.Value]
			if !ok {
				report(key, known)
				continue
			}
			walkKnownKeys(val, ft, report)
		}
	case reflect.Map:
		if n.Kind != yaml.MappingNode {
			return
		}
		for i := 1; i < len(n.Content); i += 2 {
			walkKnownKeys(n.Content[i], t.Elem(), report)
		}
	case reflect.Slice, reflect.Array:
		if n.Kind != yaml.SequenceNode {
			return
		}
		for _, item := range n.Content {
			walkKnownKeys(item, t.Elem(), report)
		}
	}
}

// yamlFields maps each YAML key of struct type t to its field type, following
// the same naming rules as yaml.v3 (tag name, else the lower-cased field name;
// "-" skipped; ",inline" flattened).
func yamlFields(t reflect.Type) map[string]reflect.Type {
	out := map[string]reflect.Type{}
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		tag := f.Tag.Get("yaml")
		if tag == "-" {
			continue
		}
		name, opts, _ := strings.Cut(tag, ",")
		if strings.Contains(","+opts+",", ",inline,") {
			ft := f.Type
			for ft.Kind() == reflect.Pointer {
				ft = ft.Elem()
			}
			if ft.Kind() == reflect.Struct {
				for k, v := range yamlFields(ft) {
					out[k] = v
				}
			}
			continue
		}
		if name == "" {
			name = strings.ToLower(f.Name)
		}
		out[name] = f.Type
	}
	return out
}

// suggest returns the known key closest to key, or "" when none is near
// enough to be a plausible typo.
func suggest(key string, known []string) string {
	best, bestDist := "", 0
	for _, k := range known {
		d := editDistance(strings.ToLower(key), strings.ToLower(k))
		if best == "" || d < bestDist {
			best, bestDist = k, d
		}
	}
	limit := 2
	if len(key) > 8 {
		limit = 3
	}
	if best != "" && bestDist <= limit && bestDist < len(key) {
		return best
	}
	return ""
}

// editDistance is the Damerau-style (adjacent transposition counts as one)
// Levenshtein distance between a and b.
func editDistance(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	prev2 := make([]int, len(rb)+1)
	prev := make([]int, len(rb)+1)
	cur := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		cur[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
			if i > 1 && j > 1 && ra[i-1] == rb[j-2] && ra[i-2] == rb[j-1] {
				cur[j] = min(cur[j], prev2[j-2]+1)
			}
		}
		prev2, prev, cur = prev, cur, prev2
	}
	return prev[len(rb)]
}

var yamlLine = regexp.MustCompile(`line (\d+): `)

// formatParseError rewrites a yaml.v3 error as "<path>:<line>: <message>",
// one per line for a type error that lists several.
func formatParseError(path string, err error) string {
	msg := strings.TrimPrefix(err.Error(), "yaml: ")
	var out []string
	if te, ok := err.(*yaml.TypeError); ok {
		for _, e := range te.Errors {
			out = append(out, locate(path, e))
		}
		return strings.Join(out, "\n")
	}
	return locate(path, msg)
}

func locate(path, msg string) string {
	msg = strings.TrimPrefix(msg, "yaml: ")
	if m := yamlLine.FindStringSubmatchIndex(msg); m != nil && m[0] == 0 {
		return fmt.Sprintf("%s:%s: %s", path, msg[m[2]:m[3]], msg[m[1]:])
	}
	return fmt.Sprintf("%s: %s", path, msg)
}
