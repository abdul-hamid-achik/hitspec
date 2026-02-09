package serve

import (
	"net/http"
	"path/filepath"
	"sort"
	"strings"

	"github.com/abdul-hamid-achik/hitspec/packages/core/parser"
)

// fileEntry is a flat file with its relative path and request count.
type fileEntry struct {
	relPath      string
	requestCount int
}

func (s *Server) handleGetWorkspace(w http.ResponseWriter, r *http.Request) {
	files, err := collectHitspecFiles(s.config.WorkDir)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	totalRequests := 0
	entries := make([]fileEntry, 0, len(files))
	for _, f := range files {
		rel, _ := filepath.Rel(s.config.WorkDir, f)
		if rel == "" {
			rel = f
		}

		reqCount := 0
		parsed, perr := parser.ParseFile(f)
		if perr == nil {
			reqCount = len(parsed.Requests)
		}
		totalRequests += reqCount
		entries = append(entries, fileEntry{relPath: rel, requestCount: reqCount})
	}

	tree := buildFileTree(entries)

	writeJSON(w, http.StatusOK, WorkspaceDTO{
		Root:          s.config.WorkDir,
		Files:         tree,
		TotalRequests: totalRequests,
		Environment:   s.config.Env,
		HasConfig:     s.fileConfig != nil,
	})
}

// buildFileTree converts a flat list of relative file paths into a nested tree.
func buildFileTree(entries []fileEntry) []FileTreeNodeDTO {
	dirs := map[string]*FileTreeNodeDTO{}

	// Ensure all ancestor directories exist
	for _, e := range entries {
		dir := filepath.Dir(e.relPath)
		d := dir
		for d != "." && d != "" {
			if _, ok := dirs[d]; !ok {
				parentDir := filepath.Dir(d)
				if parentDir == "." {
					parentDir = ""
				}
				dirs[d] = &FileTreeNodeDTO{
					Path:  d,
					Name:  filepath.Base(d),
					Dir:   parentDir,
					IsDir: true,
				}
			}
			d = filepath.Dir(d)
			if d == "." {
				break
			}
		}
	}

	// Add file nodes to their parent directory (or mark as top-level)
	type rootFile struct {
		node FileTreeNodeDTO
	}
	var topLevelFiles []FileTreeNodeDTO

	for _, e := range entries {
		dir := filepath.Dir(e.relPath)
		dirLabel := dir
		if dirLabel == "." {
			dirLabel = ""
		}

		node := FileTreeNodeDTO{
			Path:         e.relPath,
			Name:         filepath.Base(e.relPath),
			Dir:          dirLabel,
			IsDir:        false,
			RequestCount: e.requestCount,
		}

		if dir == "." {
			topLevelFiles = append(topLevelFiles, node)
		} else if parent, ok := dirs[dir]; ok {
			parent.Children = append(parent.Children, node)
		}
	}

	// Nest subdirectories into their parents (deepest first)
	sortedDirs := make([]string, 0, len(dirs))
	for d := range dirs {
		sortedDirs = append(sortedDirs, d)
	}
	sort.Slice(sortedDirs, func(i, j int) bool {
		return strings.Count(sortedDirs[i], string(filepath.Separator)) >
			strings.Count(sortedDirs[j], string(filepath.Separator))
	})

	for _, d := range sortedDirs {
		node := dirs[d]
		sortTreeNodes(node.Children)

		parentDir := filepath.Dir(d)
		if parentDir == "." || parentDir == "" {
			continue
		}
		if parent, ok := dirs[parentDir]; ok {
			parent.Children = append(parent.Children, *node)
		}
	}

	// Collect root-level items
	var root []FileTreeNodeDTO

	for _, d := range sortedDirs {
		parentDir := filepath.Dir(d)
		if parentDir == "." || parentDir == "" {
			node := dirs[d]
			sortTreeNodes(node.Children)
			root = append(root, *node)
		}
	}

	root = append(root, topLevelFiles...)
	sortTreeNodes(root)
	return root
}

// sortTreeNodes sorts: directories first, then files, alphabetically within each.
func sortTreeNodes(nodes []FileTreeNodeDTO) {
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].IsDir != nodes[j].IsDir {
			return nodes[i].IsDir
		}
		return strings.ToLower(nodes[i].Name) < strings.ToLower(nodes[j].Name)
	})
}
