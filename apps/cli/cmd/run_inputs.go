package cmd

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/abdul-hamid-achik/hitspec/v2/internal/watchutil"
	"github.com/spf13/cobra"
)

// stdinFileName is the name `hitspec run -` gives the document it reads from
// stdin. It is a relative name, so the document's own relative paths
// (environments, @file bodies, snapshots) resolve against the working directory.
const stdinFileName = "<stdin>"

// maxStdinBytes bounds what `hitspec run -` reads: a .http file is text, so
// anything near this size is a mistake (a binary piped by accident).
const maxStdinBytes = 32 << 20

// readStdinSource reads the whole document for `hitspec run -`.
func readStdinSource(r io.Reader) ([]byte, error) {
	src, err := io.ReadAll(io.LimitReader(r, maxStdinBytes+1))
	if err != nil {
		return nil, fmt.Errorf("cannot read stdin: %w", err)
	}
	if len(src) > maxStdinBytes {
		return nil, fmt.Errorf("stdin is larger than %d MiB; expected one .http file", maxStdinBytes>>20)
	}
	if strings.TrimSpace(string(src)) == "" {
		return nil, fmt.Errorf("no input on stdin (expected one .http file)")
	}
	return src, nil
}

// resolveRunInputs turns the arguments of `hitspec run` into the files to run.
// `-` reads one document from stdin instead (returned as src, with the single
// file name stdinFileName); it cannot be combined with other arguments, --watch
// or --stress, which all need real files.
func resolveRunInputs(cmd *cobra.Command, args []string) (files []string, src []byte, err error) {
	stdin := false
	for _, a := range args {
		stdin = stdin || a == "-"
	}
	if !stdin {
		files, err = collectFiles(args)
		return files, nil, err
	}

	usage := func(msg string) error {
		fmt.Fprintf(cmd.ErrOrStderr(), "Error: %s\n", msg)
		return newExitError(ExitUsageError, msg)
	}
	switch {
	case len(args) > 1:
		return nil, nil, usage("'-' (read from stdin) cannot be combined with other files")
	case watchFlag:
		return nil, nil, usage("--watch cannot be used with '-' (stdin has no file to watch)")
	case stressFlag:
		return nil, nil, usage("--stress cannot be used with '-' (stress mode needs files)")
	}
	src, err = readStdinSource(cmd.InOrStdin())
	if err != nil {
		return nil, nil, err
	}
	return []string{stdinFileName}, src, nil
}

// collectFiles resolves the file arguments of `hitspec run` into the list of
// files to run:
//
//   - a file is used as is (it must be .http or .hitspec);
//   - a directory contributes every .http/.hitspec file below it, except under
//     .git, node_modules and vendor (a directory named explicitly is still
//     walked, and so is an explicit path inside one of them);
//   - an argument with glob characters that names no existing path is a
//     pattern: `*`, `?` and `[...]` match within one path segment and `**`
//     matches any number of directories, so quoted patterns like
//     'tests/**/*.http' work without help from the shell;
//   - a file named more than once, directly or through a directory or a
//     pattern, runs once, at its first position.
func collectFiles(args []string) ([]string, error) {
	var files []string
	seen := make(map[string]bool)
	add := func(file string) {
		key := file
		if abs, err := filepath.Abs(file); err == nil {
			key = abs
		}
		if seen[key] {
			return
		}
		seen[key] = true
		files = append(files, file)
	}

	for _, arg := range args {
		info, err := os.Stat(arg)
		if err != nil {
			if hasGlobMeta(arg) && (os.IsNotExist(err) || runtime.GOOS == "windows") {
				// Windows rejects '*' and '?' in a path with "filename syntax is
				// incorrect" instead of "not found": still a pattern, not an error.
				matches, gerr := expandGlob(arg)
				if gerr != nil {
					return nil, gerr
				}
				if len(matches) == 0 {
					return nil, fmt.Errorf("no .http or .hitspec files match %q", arg)
				}
				for _, m := range matches {
					add(m)
				}
				continue
			}
			if os.IsNotExist(err) {
				return nil, fmt.Errorf("file or directory not found: %s", arg)
			}
			return nil, fmt.Errorf("cannot access %s: %w", arg, err)
		}

		if info.IsDir() {
			dirFiles, err := walkHitspecFiles(arg)
			if err != nil {
				return nil, err
			}
			for _, f := range dirFiles {
				add(f)
			}
			continue
		}
		if !isHitspecFile(arg) {
			return nil, fmt.Errorf("%s is not a .http or .hitspec file", arg)
		}
		add(arg)
	}

	return files, nil
}

// walkHitspecFiles lists the hitspec files below root in lexical order,
// skipping directories nobody keeps tests in (watchutil.SkipDir) unless root
// itself is one.
func walkHitspecFiles(root string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if p != root && watchutil.SkipDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if isHitspecFile(p) {
			files = append(files, p)
		}
		return nil
	})
	return files, err
}

func hasGlobMeta(s string) bool { return strings.ContainsAny(s, "*?[") }

// expandGlob returns the hitspec files matching pattern, in lexical order.
func expandGlob(pattern string) ([]string, error) {
	slashed := filepath.ToSlash(pattern)
	segs := strings.Split(slashed, "/")
	for _, seg := range segs {
		if seg == "**" {
			continue
		}
		if _, err := path.Match(seg, ""); err != nil {
			return nil, fmt.Errorf("invalid pattern %q: %w", pattern, err)
		}
	}

	// The static leading segments are where the walk starts.
	n := 0
	for n < len(segs) && !hasGlobMeta(segs[n]) {
		n++
	}
	root := strings.Join(segs[:n], "/")
	switch {
	case n == 1 && segs[0] == "": // "/*.http": the filesystem root
		root = "/"
	case root == "":
		root = "."
	}
	rest := segs[n:]
	deep := false // does the pattern reach below its own depth?
	for _, seg := range rest {
		deep = deep || seg == "**"
	}

	var matches []string
	err := filepath.WalkDir(filepath.FromSlash(root), func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // unreadable directories are skipped, like the shell
		}
		rel, rerr := filepath.Rel(filepath.FromSlash(root), p)
		if rerr != nil || rel == "." {
			return nil
		}
		relSegs := strings.Split(filepath.ToSlash(rel), "/")
		if d.IsDir() {
			if watchutil.SkipDir(d.Name()) {
				return filepath.SkipDir
			}
			// A directory the pattern names stands for everything below it.
			if matchSegments(rest, relSegs) {
				sub, werr := walkHitspecFiles(p)
				if werr != nil {
					return nil
				}
				matches = append(matches, sub...)
				return filepath.SkipDir
			}
			if !deep && len(relSegs) >= len(rest) {
				return filepath.SkipDir // nothing below can match
			}
			return nil
		}
		if isHitspecFile(p) && matchSegments(rest, relSegs) {
			matches = append(matches, p)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(matches)
	return matches, nil
}

// matchSegments matches path segments against pattern segments, where "**"
// matches zero or more whole segments and every other pattern segment is a
// path.Match pattern.
func matchSegments(pat, segs []string) bool {
	for len(pat) > 0 {
		if pat[0] == "**" {
			for len(pat) > 1 && pat[1] == "**" {
				pat = pat[1:]
			}
			if len(pat) == 1 {
				return true
			}
			for k := 0; k <= len(segs); k++ {
				if matchSegments(pat[1:], segs[k:]) {
					return true
				}
			}
			return false
		}
		if len(segs) == 0 {
			return false
		}
		if ok, _ := path.Match(pat[0], segs[0]); !ok {
			return false
		}
		pat, segs = pat[1:], segs[1:]
	}
	return len(segs) == 0
}
