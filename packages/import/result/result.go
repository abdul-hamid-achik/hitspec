// Package result holds the value every importer returns: the generated
// .http content plus the things that could not be carried over faithfully.
package result

import (
	"fmt"
	"sort"
	"strings"

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
