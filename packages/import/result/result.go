// Package result holds the value every importer returns: the generated
// .http content plus the things that could not be carried over faithfully.
package result

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"gopkg.in/yaml.v3"
)

// Result is the outcome of converting a foreign API definition.
type Result struct {
	// Content is the generated .http file.
	Content string
	// Warnings lists every construct that was dropped or approximated. An
	// importer must never lose input without adding an entry here.
	Warnings []string
	// Environments holds per-environment variable sets destined for the
	// `environments:` section of hitspec.yaml. It is only used when the source
	// has more than one environment; a single variable set is emitted as
	// `@name = value` lines in Content.
	Environments map[string]map[string]any
	// DefaultEnvironment names the environment to select by default, if any.
	DefaultEnvironment string
}

// Warn records a warning once.
func (r *Result) Warn(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	for _, w := range r.Warnings {
		if w == msg {
			return
		}
	}
	r.Warnings = append(r.Warnings, msg)
}

// EnvironmentNames returns the environment names in sorted order.
func (r *Result) EnvironmentNames() []string {
	names := make([]string, 0, len(r.Environments))
	for n := range r.Environments {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// HasEnvironments reports whether the result carries a hitspec.yaml fragment.
func (r *Result) HasEnvironments() bool {
	return len(r.Environments) > 0
}

// EnvironmentsYAML renders Environments as a hitspec.yaml document with a
// stable key order.
func (r *Result) EnvironmentsYAML() (string, error) {
	root := &yaml.Node{Kind: yaml.MappingNode}
	if r.DefaultEnvironment != "" {
		root.Content = append(root.Content, scalar("defaultEnvironment"), scalar(r.DefaultEnvironment))
	}
	envs := &yaml.Node{Kind: yaml.MappingNode}
	names := make([]string, 0, len(r.Environments))
	for n := range r.Environments {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		vars := &yaml.Node{Kind: yaml.MappingNode}
		keys := make([]string, 0, len(r.Environments[n]))
		for k := range r.Environments[n] {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			v := &yaml.Node{}
			if err := v.Encode(r.Environments[n][k]); err != nil {
				return "", err
			}
			vars.Content = append(vars.Content, scalar(k), v)
		}
		envs.Content = append(envs.Content, scalar(n), vars)
	}
	root.Content = append(root.Content, scalar("environments"), envs)
	out, err := yaml.Marshal(root)
	if err != nil {
		return "", err
	}
	return strings.TrimRight(string(out), "\n") + "\n", nil
}

func scalar(s string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Value: s}
}

// ContentWithEnvironments returns Content followed, when the result carries
// environments, by a commented-out hitspec.yaml block. Callers that can only
// return one string (the TUI and REST import) use it so no variable is lost.
func (r *Result) ContentWithEnvironments() string {
	if !r.HasEnvironments() {
		return r.Content
	}
	y, err := r.EnvironmentsYAML()
	if err != nil {
		return r.Content
	}
	var sb strings.Builder
	sb.WriteString(r.Content)
	if !strings.HasSuffix(r.Content, "\n") {
		sb.WriteString("\n")
	}
	sb.WriteString("\n# Environments: add this to hitspec.yaml\n")
	for _, line := range strings.Split(strings.TrimRight(y, "\n"), "\n") {
		sb.WriteString("# " + line + "\n")
	}
	return sb.String()
}

// Scalar renders a decoded JSON/YAML value as text. Numbers decode as float64,
// which fmt would print as 1.234567e+06 for a 7-digit ID; they are written in
// plain decimal instead. Maps and slices become JSON.
func Scalar(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(t), 'f', -1, 32)
	case map[string]any, []any, map[any]any:
		if b, err := json.Marshal(t); err == nil {
			return string(b)
		}
	}
	return fmt.Sprint(v)
}

// Normalize turns whole-number float64 values (how JSON decodes every number)
// into int64 so YAML writes 1234567 instead of 1.234567e+06.
func Normalize(v any) any {
	if f, ok := v.(float64); ok && f == math.Trunc(f) && math.Abs(f) < 1e15 {
		return int64(f)
	}
	return v
}

// CanNameMultipartFile reports whether name fits the `file <name> @<path>` line
// of a >>>multipart block: one word, not starting with "@".
func CanNameMultipartFile(name string) bool {
	if name == "" || strings.HasPrefix(name, "@") {
		return false
	}
	return !strings.ContainsFunc(name, unicode.IsSpace)
}

// MultipartFileLine renders a multipart file part so the form field name
// survives a round trip: `file <name> @<path>`. A name that cannot be written
// that way (see CanNameMultipartFile) gives the unnamed `file @<path>`, which
// hitspec sends as field "file"; the caller should warn then.
func MultipartFileLine(name, path string) string {
	if CanNameMultipartFile(name) {
		return "file " + name + " @" + path
	}
	return "file @" + path
}
