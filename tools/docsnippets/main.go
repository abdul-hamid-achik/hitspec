// Command docsnippets extracts every ```http (or ```hitspec) fenced block from
// the project's Markdown/MDX documentation and checks it with the real parser,
// so documentation cannot drift into syntax hitspec does not have.
//
//	go run ./tools/docsnippets            # check the default set of files
//	go run ./tools/docsnippets docs/a.mdx # check specific files or directories
//
// A block is flagged when the parser rejects it, when it defines a request
// with no URL, or when it uses an @directive the parser does not know (the
// parser stores unknown directives as "custom annotations", so a typo such as
// `@assert` or `@capture` would otherwise be accepted silently). Directives in
// the contract.* and x-* namespaces are legitimate extension points.
//
// Blocks that are intentionally partial or invalid are exempted with an
// explicit marker on the line directly above the fence:
//
//	{/* docsnippets: skip */}          (in .mdx files)
//	<!-- docsnippets: skip -->         (in .md and .txt files)
//
// Fragments (a lone header or body, no request line) are accepted as long as
// the parser does not reject them.
//
// With -bin <hitspec binary> the tool also checks shell blocks: every long
// flag on a `hitspec <command>` line must appear in that command's --help
// output, so a documented `--fail-fast` that does not exist is caught. The
// same skip marker exempts a shell block.
package main

import (
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/abdul-hamid-achik/hitspec/v2/packages/core/parser"
)

// defaultTargets are checked when no arguments are given. llms.txt is
// embedded in the CLI (apps/cli/cmd/llms.txt is a byte-identical copy), so
// only the root copy is listed.
var defaultTargets = []string{
	"apps/docs",
	"llms.txt",
	"README.md",
	"AGENTS.md",
	"CONTRIBUTING.md",
	"examples",
}

var (
	fenceOpen = regexp.MustCompile("^(\\s*)(`{3,}|~{3,})\\s*([A-Za-z0-9_+-]*)")
	skipMark  = regexp.MustCompile(`docsnippets:\s*skip`)
	// varDef is a file-level variable definition (`@baseUrl = value`). Any
	// other line starting with "@" is not hitspec syntax: directives live in
	// comments (`# @name x`), and the parser would silently swallow such a
	// line as part of the request body.
	varDef = regexp.MustCompile(`^@[A-Za-z_][A-Za-z0-9_.-]*\s*=`)
	// intDirective is a directive whose value must be an integer. The parser
	// warns about a bad @timeout/@retry/@retryDelay, but silently ignores a bad
	// @stress.weight/@stress.think (e.g. `@stress.think 500ms`).
	intDirective = regexp.MustCompile(`^#\s*@(stress\.weight|stress\.think)\s+(\S.*)$`)

	// requestLine matches a request line; a snippet without one is a
	// fragment (just an assertion block, say) and is checked inside a
	// placeholder request.
	requestLine = regexp.MustCompile(`^(GET|POST|PUT|PATCH|DELETE|HEAD|OPTIONS|TRACE|CONNECT)\s+\S`)
)

// snippet is one fenced block.
type snippet struct {
	file string
	line int    // 1-based line of the opening fence
	lang string // lower-cased fence language
	body string
	skip bool
}

func main() {
	verbose := flag.Bool("v", false, "list every checked block")
	bin := flag.String("bin", "", "hitspec binary; also check long flags in shell blocks against its --help")
	flag.Parse()
	flags := newFlagChecker(*bin)

	targets := flag.Args()
	if len(targets) == 0 {
		targets = defaultTargets
	}

	files, err := collect(targets)
	if err != nil {
		fmt.Fprintln(os.Stderr, "docsnippets:", err)
		os.Exit(2)
	}

	var checked, skipped, failed int
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			fmt.Fprintln(os.Stderr, "docsnippets:", err)
			os.Exit(2)
		}
		for _, s := range extract(f, string(data)) {
			if s.skip {
				skipped++
				continue
			}
			checked++
			var problems []string
			if isShell(s.lang) {
				problems = flags.check(s)
			} else {
				problems = check(s)
			}
			if len(problems) > 0 {
				failed++
				for _, p := range problems {
					fmt.Printf("%s:%d: %s\n", s.file, s.line, p)
				}
			} else if *verbose {
				fmt.Printf("%s:%d: ok\n", s.file, s.line)
			}
		}
	}

	fmt.Printf("docsnippets: %d blocks checked, %d skipped, %d failed\n", checked, skipped, failed)
	if failed > 0 {
		os.Exit(1)
	}
}

// collect expands targets into a sorted list of .md/.mdx/.txt files.
func collect(targets []string) ([]string, error) {
	var out []string
	for _, t := range targets {
		info, err := os.Stat(t)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			out = append(out, t)
			continue
		}
		err = filepath.WalkDir(t, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if d.Name() == "node_modules" || d.Name() == ".git" {
					return filepath.SkipDir
				}
				return nil
			}
			switch strings.ToLower(filepath.Ext(path)) {
			case ".md", ".mdx", ".txt":
				out = append(out, path)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	sort.Strings(out)
	return out, nil
}

// extract returns every http/hitspec/shell fenced block in a document.
func extract(file, doc string) []snippet {
	lines := strings.Split(doc, "\n")
	var out []snippet
	for i := 0; i < len(lines); i++ {
		m := fenceOpen.FindStringSubmatch(lines[i])
		if m == nil {
			continue
		}
		indent, fence, lang := m[1], m[2], strings.ToLower(m[3])
		start := i
		var body []string
		for i++; i < len(lines); i++ {
			trimmed := strings.TrimSpace(lines[i])
			if strings.HasPrefix(trimmed, fence) && strings.Trim(trimmed, fence[:1]) == "" {
				break
			}
			body = append(body, strings.TrimPrefix(lines[i], indent))
		}
		if lang != "http" && lang != "hitspec" && !isShell(lang) {
			continue
		}
		skip := false
		for j := start - 1; j >= 0; j-- {
			if strings.TrimSpace(lines[j]) == "" {
				continue
			}
			skip = skipMark.MatchString(lines[j])
			break
		}
		out = append(out, snippet{file: file, line: start + 1, lang: lang, body: strings.Join(body, "\n") + "\n", skip: skip})
	}
	return out
}

// isShell reports whether a fence language holds shell commands (YAML is
// included for CI workflow samples with `run: hitspec ...` steps).
func isShell(lang string) bool {
	switch lang {
	case "bash", "sh", "shell", "yaml", "yml":
		return true
	}
	return false
}

// customOK reports whether a custom (unrecognized-by-the-parser) directive is
// an intentional extension point.
func customOK(name string) bool {
	return strings.HasPrefix(name, "contract.") || strings.HasPrefix(name, "x-")
}

// placeholderRequest is the request a fragment is attached to.
const placeholderRequest = "GET http://docsnippets.invalid/"

// wrapFragment returns body unchanged when it has a request line. Otherwise it
// is a fragment: the leading comment, directive and variable lines stay on
// top, a placeholder request follows them, and the rest (blocks, headers)
// comes after it, so directives and >>> blocks have a request to belong to.
func wrapFragment(body string) string {
	lines := strings.Split(body, "\n")
	for _, line := range lines {
		if requestLine.MatchString(strings.TrimSpace(line)) {
			return body
		}
	}
	split := 0
	for split < len(lines) {
		t := strings.TrimSpace(lines[split])
		if t != "" && !strings.HasPrefix(t, "#") && !strings.HasPrefix(t, "//") && !varDef.MatchString(t) {
			break
		}
		split++
	}
	out := append([]string{}, lines[:split]...)
	out = append(out, placeholderRequest)
	if split < len(lines) && !strings.HasPrefix(strings.TrimSpace(lines[split]), ">>>") {
		// Header-like lines go right under the request; anything else (a
		// body, a block) after a blank line.
		if !strings.Contains(lines[split], ":") {
			out = append(out, "")
		}
	} else {
		out = append(out, "")
	}
	out = append(out, lines[split:]...)
	return strings.Join(out, "\n")
}

// check returns the problems found in one snippet.
func check(s snippet) []string {
	if s.lang == "hitspec" {
		s.lang = "http"
	}
	s.body = wrapFragment(s.body)
	parsed, warnings, err := parseCapturing(s)
	if err != nil {
		return []string{"parse error: " + oneLine(err.Error())}
	}
	var problems []string
	for i, line := range strings.Split(s.body, "\n") {
		if m := intDirective.FindStringSubmatch(strings.TrimSpace(line)); m != nil && !isDigits(strings.TrimSpace(m[2])) {
			problems = append(problems, fmt.Sprintf("line %d: @%s needs an integer, got %q", i+1, m[1], strings.TrimSpace(m[2])))
		}
		if t := strings.TrimSpace(line); strings.HasPrefix(t, "@") && !varDef.MatchString(t) {
			problems = append(problems, fmt.Sprintf("line %d: %q is not hitspec syntax (directives are comments such as `# @name x`; variables are `@name = value`)", i+1, oneLine(t)))
		}
	}
	for _, w := range warnings {
		problems = append(problems, "parser warning: "+w)
	}
	for _, req := range parsed.Requests {
		label := req.Name
		if label == "" {
			label = req.Method + " " + req.URL
		}
		if req.Method != "" && strings.TrimSpace(req.URL) == "" {
			problems = append(problems, fmt.Sprintf("request %q has an empty URL", label))
		}
		if req.Metadata == nil {
			continue
		}
		var unknown []string
		for name := range req.Metadata.Custom {
			if !customOK(name) {
				unknown = append(unknown, "@"+name)
			}
		}
		sort.Strings(unknown)
		if len(unknown) > 0 {
			problems = append(problems, fmt.Sprintf("request %q uses unknown directive(s) %s", label, strings.Join(unknown, ", ")))
		}
	}
	return problems
}

// parseCapturing parses a snippet. The parser reports recoverable problems
// (an @timeout that is not a number, say) as "warning:" lines on stderr and
// carries on; those are documentation bugs too, so stderr is captured during
// the parse and returned as warnings. Not safe for concurrent use.
func parseCapturing(s snippet) (*parser.File, []string, error) {
	r, w, err := os.Pipe()
	if err != nil {
		return nil, nil, err
	}
	orig := os.Stderr
	os.Stderr = w
	done := make(chan string)
	go func() {
		b, _ := io.ReadAll(r)
		done <- string(b)
	}()

	parsed, perr := parser.Parse(s.body, fmt.Sprintf("%s:%d", s.file, s.line))

	os.Stderr = orig
	_ = w.Close()
	captured := <-done
	_ = r.Close()

	var warnings []string
	for _, line := range strings.Split(captured, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			warnings = append(warnings, strings.TrimPrefix(line, "warning: "))
		}
	}
	// Parse itself does not print: the parser collects its warnings (unknown
	// directives, bad @timeout values, ...) in File.Warnings.
	if parsed != nil {
		for _, w := range parsed.Warnings {
			warnings = append(warnings, w.String())
		}
	}
	return parsed, warnings, perr
}

func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// flagChecker validates `hitspec ...` command lines in shell blocks against
// the real binary's --help. It is a no-op without a binary.
type flagChecker struct {
	bin   string
	cache map[string]helpInfo
}

type helpInfo struct {
	flags map[string]bool
	subs  map[string]bool
	ok    bool
}

var (
	longFlag   = regexp.MustCompile(`--([A-Za-z][A-Za-z0-9-]*)`)
	subCommand = regexp.MustCompile(`^\s{2}([a-z][a-z0-9-]*)\s`)
)

func newFlagChecker(bin string) *flagChecker {
	return &flagChecker{bin: bin, cache: map[string]helpInfo{}}
}

// help returns the flags and subcommands advertised by `hitspec <path> --help`.
func (c *flagChecker) help(path []string) helpInfo {
	key := strings.Join(path, " ")
	if h, ok := c.cache[key]; ok {
		return h
	}
	h := helpInfo{flags: map[string]bool{}, subs: map[string]bool{}}
	out, err := exec.Command(c.bin, append(append([]string{}, path...), "--help")...).CombinedOutput() //nolint:gosec // developer tool: the binary is given on the command line
	if err == nil {
		h.ok = true
		inCommands := false
		for _, line := range strings.Split(string(out), "\n") {
			switch {
			case strings.HasPrefix(line, "Available Commands:"):
				inCommands = true
			case strings.TrimSpace(line) == "":
				inCommands = false
			case inCommands:
				if m := subCommand.FindStringSubmatch(line); m != nil {
					h.subs[m[1]] = true
				}
			}
			for _, m := range longFlag.FindAllStringSubmatch(line, -1) {
				h.flags[m[1]] = true
			}
		}
	}
	c.cache[key] = h
	return h
}

// check flags unknown commands and unknown long flags on `hitspec` lines.
func (c *flagChecker) check(s snippet) []string {
	if c.bin == "" {
		return nil
	}
	var problems []string
	// Join backslash continuations so a flag on a continuation line is seen.
	text := strings.ReplaceAll(s.body, "\\\n", " ")
	for i, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "$ "))
		line = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(line, "- "), "run:"))
		if !strings.HasPrefix(line, "hitspec ") {
			continue
		}
		// Only the hitspec command itself: stop at pipes, redirects, chains.
		if cut := strings.IndexAny(line, "|><;&#"); cut >= 0 {
			line = line[:cut]
		}
		toks := strings.Fields(line)[1:]
		// Everything after a bare "--" belongs to another program.
		for k, t := range toks {
			if t == "--" {
				toks = toks[:k]
				break
			}
		}
		var path []string
		for _, t := range toks {
			if strings.HasPrefix(t, "-") || strings.ContainsAny(t, "./\"'$=<>") {
				break
			}
			// Walk down only while the token is a known subcommand.
			if !c.help(path).subs[t] {
				break
			}
			path = append(path, t)
		}
		if len(toks) > 0 && !strings.HasPrefix(toks[0], "-") && len(path) == 0 && !strings.ContainsAny(toks[0], "./\"'$=<>") {
			problems = append(problems, fmt.Sprintf("line %d: `hitspec %s` is not a command", i+1, toks[0]))
			continue
		}
		h := c.help(path)
		for k, t := range toks {
			// --timeout takes a duration string; a bare number is rejected.
			if t == "--timeout" && k+1 < len(toks) && isDigits(toks[k+1]) {
				problems = append(problems, fmt.Sprintf("line %d: --timeout %s needs a unit (e.g. 30s or 1m), a bare number is not a duration", i+1, toks[k+1]))
			}
			if m := longFlag.FindStringSubmatch(t); m != nil && strings.HasPrefix(t, "--") {
				if !h.flags[strings.SplitN(m[1], "=", 2)[0]] {
					problems = append(problems, fmt.Sprintf("line %d: `hitspec %s` has no flag --%s", i+1, strings.Join(path, " "), m[1]))
				}
			}
		}
	}
	return problems
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
