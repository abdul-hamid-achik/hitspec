package http

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/abdul-hamid-achik/hitspec/v2/packages/core/parser"
)

// CheckFileRefs verifies that every local file a request reads (a `< file` body
// and multipart file parts) lives under root once symlinks are resolved. It is
// for callers that run files on behalf of someone else (hitspec serve, the
// studio) and must not let `< /etc/passwd` read outside the workspace. resolve
// expands {{variables}} exactly as the request builder does, and baseDir is the
// directory relative paths are joined to. An empty root disables the check.
func CheckFileRefs(req *parser.Request, resolve func(string) string, baseDir, root string) error {
	if root == "" || req == nil || req.Body == nil {
		return nil
	}
	var refs []string
	if req.Body.ContentType == parser.BodyFile && req.Body.FilePath != "" {
		refs = append(refs, req.Body.FilePath)
	}
	if req.Body.ContentType == parser.BodyMultipart {
		for _, f := range req.Body.Multipart {
			if f.Type == parser.MultipartFieldFile && f.Path != "" {
				refs = append(refs, f.Path)
			}
		}
	}
	if len(refs) == 0 {
		return nil
	}
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return fmt.Errorf("resolve workspace: %w", err)
	}
	for _, ref := range refs {
		p := resolve(ref)
		if !filepath.IsAbs(p) && baseDir != "" {
			p = filepath.Join(baseDir, p)
		}
		if !withinRoot(realRoot, p) {
			return fmt.Errorf("file reference %q is outside the workspace", ref)
		}
	}
	return nil
}

// CheckPathWithinRoot verifies that path lives under root once symlinks are
// resolved; it is CheckFileRefs for a single path (a schema file named by an
// assertion). An empty root disables the check.
func CheckPathWithinRoot(root, path string) error {
	if root == "" {
		return nil
	}
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return fmt.Errorf("resolve workspace: %w", err)
	}
	if !withinRoot(realRoot, path) {
		return fmt.Errorf("file reference %q is outside the workspace", path)
	}
	return nil
}

// withinRoot reports whether path, with symlinks resolved, is root or below it.
// A path that does not exist yet is checked through its nearest existing
// ancestor so a dangling symlink cannot be used to escape.
func withinRoot(realRoot, path string) bool {
	path = filepath.Clean(path)
	var tail []string
	cur := path
	for {
		real, err := filepath.EvalSymlinks(cur)
		if err == nil {
			for i := len(tail) - 1; i >= 0; i-- {
				real = filepath.Join(real, tail[i])
			}
			rel, err := filepath.Rel(realRoot, real)
			return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
		}
		parent := filepath.Dir(cur)
		if parent == cur || !os.IsNotExist(err) {
			return false
		}
		tail = append(tail, filepath.Base(cur))
		cur = parent
	}
}
