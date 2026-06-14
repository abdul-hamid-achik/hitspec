package clientmgr

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/abdul-hamid-achik/hitspec/packages/core/parser"
)

type fileEntry struct {
	relPath      string
	requestCount int
}

// Workspace returns workspace metadata and a file tree.
func (m *Manager) Workspace(ctx context.Context) (WorkspaceDTO, error) {
	_ = ctx
	files, err := collectHitspecFiles(m.config.WorkDir)
	if err != nil {
		return WorkspaceDTO{}, err
	}

	totalRequests := 0
	entries := make([]fileEntry, 0, len(files))
	for _, f := range files {
		rel := m.relPath(f)
		reqCount := 0
		parsed, perr := parser.ParseFile(f)
		if perr == nil {
			reqCount = len(parsed.Requests)
		}
		totalRequests += reqCount
		entries = append(entries, fileEntry{relPath: rel, requestCount: reqCount})
	}

	m.configMu.RLock()
	env := m.config.Env
	hasConfig := m.fileConfig != nil
	m.configMu.RUnlock()

	return WorkspaceDTO{
		Root:          m.config.WorkDir,
		Files:         buildFileTree(entries),
		TotalRequests: totalRequests,
		Environment:   env,
		HasConfig:     hasConfig,
	}, nil
}

// ListFiles returns all .http/.hitspec files in the workspace.
func (m *Manager) ListFiles(ctx context.Context) ([]FileInfoDTO, error) {
	_ = ctx
	files, err := collectHitspecFiles(m.config.WorkDir)
	if err != nil {
		return nil, err
	}
	dtos := make([]FileInfoDTO, 0, len(files))
	for _, f := range files {
		info, err := os.Stat(f)
		if err != nil {
			continue
		}
		reqCount := 0
		parsed, err := parser.ParseFile(f)
		if err == nil {
			reqCount = len(parsed.Requests)
		}
		dtos = append(dtos, FileInfoDTO{
			Path:         f,
			RelativePath: m.relPath(f),
			Name:         filepath.Base(f),
			Size:         info.Size(),
			ModTime:      info.ModTime().UTC().Format("2006-01-02T15:04:05Z"),
			RequestCount: reqCount,
		})
	}
	return dtos, nil
}

// GetFile parses and returns a hitspec file.
func (m *Manager) GetFile(ctx context.Context, relPath string) (*ParsedFileDTO, error) {
	_ = ctx
	absPath, err := m.absPath(relPath)
	if err != nil {
		return nil, err
	}
	parsed, err := parser.ParseFile(absPath)
	if err != nil {
		return nil, err
	}
	return convertFile(parsed), nil
}

// ReadFile returns raw file content.
func (m *Manager) ReadFile(ctx context.Context, relPath string) (string, error) {
	_ = ctx
	absPath, err := m.absPath(relPath)
	if err != nil {
		return "", err
	}
	if !isHitspecFile(absPath) {
		return "", fmt.Errorf("only .http and .hitspec files can be read")
	}
	content, err := os.ReadFile(absPath)
	if err != nil {
		return "", err
	}
	if len(content) > maxRequestBody {
		return "", fmt.Errorf("file exceeds %d byte editor limit", maxRequestBody)
	}
	return string(content), nil
}

// SaveFile writes raw file content and returns the parsed file when valid.
func (m *Manager) SaveFile(ctx context.Context, relPath, content string) (*ParsedFileDTO, error) {
	_ = ctx
	if err := m.requireWritable(); err != nil {
		return nil, err
	}
	if len(content) > maxRequestBody {
		return nil, fmt.Errorf("content exceeds %d byte limit", maxRequestBody)
	}
	absPath, err := m.absPath(relPath)
	if err != nil {
		return nil, err
	}
	if !isHitspecFile(absPath) {
		return nil, fmt.Errorf("only .http and .hitspec files can be saved")
	}
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("file not found")
	}

	m.suppressWatch(absPath)
	if err := os.WriteFile(absPath, []byte(content), 0o644); err != nil {
		return nil, err
	}
	m.publish("file_changed", FileEvent{Path: m.relPath(absPath), Operation: "changed", Timestamp: nowISO()})

	parsed, err := parser.ParseFile(absPath)
	if err != nil {
		return nil, err
	}
	return convertFile(parsed), nil
}

// CreateFile creates a new hitspec file.
func (m *Manager) CreateFile(ctx context.Context, relPath, content string) (*ParsedFileDTO, error) {
	_ = ctx
	if err := m.requireWritable(); err != nil {
		return nil, err
	}
	if relPath == "" {
		return nil, fmt.Errorf("path is required")
	}
	if !isHitspecFile(relPath) && !strings.HasSuffix(relPath, ".http") && !strings.HasSuffix(relPath, ".hitspec") {
		relPath += ".http"
	}
	absPath, err := m.absPath(relPath)
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(absPath); err == nil {
		return nil, fmt.Errorf("file already exists")
	}
	if content == "" {
		content = "### New Request\nGET https://example.com\n"
	}
	if len(content) > maxRequestBody {
		return nil, fmt.Errorf("content exceeds %d byte limit", maxRequestBody)
	}
	if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
		return nil, err
	}
	m.suppressWatch(absPath)
	if err := os.WriteFile(absPath, []byte(content), 0o644); err != nil {
		return nil, err
	}
	m.publish("file_changed", FileEvent{Path: m.relPath(absPath), Operation: "created", Timestamp: nowISO()})

	parsed, err := parser.ParseFile(absPath)
	if err != nil {
		return nil, err
	}
	return convertFile(parsed), nil
}

// DeleteFile removes a hitspec file.
func (m *Manager) DeleteFile(ctx context.Context, relPath string) error {
	_ = ctx
	if err := m.requireWritable(); err != nil {
		return err
	}
	absPath, err := m.absPath(relPath)
	if err != nil {
		return err
	}
	if !isHitspecFile(absPath) {
		return fmt.Errorf("only .http and .hitspec files can be deleted")
	}
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		return fmt.Errorf("file not found")
	}
	m.suppressWatch(absPath)
	if err := os.Remove(absPath); err != nil {
		return err
	}
	m.publish("file_changed", FileEvent{Path: m.relPath(absPath), Operation: "deleted", Timestamp: nowISO()})
	return nil
}

// SearchRequests finds requests across the workspace whose name, method, URL, or
// tags contain the (case-insensitive) query. An empty query returns nil.
func (m *Manager) SearchRequests(ctx context.Context, query string) ([]SearchResultDTO, error) {
	_ = ctx
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return nil, nil
	}
	files, err := collectHitspecFiles(m.config.WorkDir)
	if err != nil {
		return nil, err
	}
	var results []SearchResultDTO
	for _, f := range files {
		parsed, perr := parser.ParseFile(f)
		if perr != nil {
			continue
		}
		rel := m.relPath(f)
		for _, r := range parsed.Requests {
			hay := strings.ToLower(strings.Join([]string{r.Name, r.Method, r.URL, strings.Join(r.Tags, " ")}, " "))
			if strings.Contains(hay, q) {
				results = append(results, SearchResultDTO{
					File:        rel,
					RequestName: r.Name,
					Method:      r.Method,
					URL:         r.URL,
					Tags:        r.Tags,
					Line:        r.Line,
				})
			}
		}
	}
	return results, nil
}

// RenameFile moves a hitspec file to a new path within the workspace.
func (m *Manager) RenameFile(ctx context.Context, oldPath, newPath string) (*ParsedFileDTO, error) {
	_ = ctx
	if err := m.requireWritable(); err != nil {
		return nil, err
	}
	srcAbs, err := m.absPath(oldPath)
	if err != nil {
		return nil, err
	}
	if !isHitspecFile(srcAbs) {
		return nil, fmt.Errorf("only .http and .hitspec files can be renamed")
	}
	if _, err := os.Stat(srcAbs); os.IsNotExist(err) {
		return nil, fmt.Errorf("file not found")
	}
	if !isHitspecFile(newPath) {
		return nil, fmt.Errorf("new path must end in .http or .hitspec")
	}
	dstAbs, err := m.absPath(newPath)
	if err != nil {
		return nil, err
	}
	if srcAbs == dstAbs {
		return nil, fmt.Errorf("source and destination are the same")
	}
	if _, err := os.Stat(dstAbs); err == nil {
		return nil, fmt.Errorf("destination already exists")
	}
	if err := os.MkdirAll(filepath.Dir(dstAbs), 0o755); err != nil {
		return nil, err
	}
	m.suppressWatch(srcAbs)
	m.suppressWatch(dstAbs)
	if err := os.Rename(srcAbs, dstAbs); err != nil {
		return nil, err
	}
	m.publish("file_changed", FileEvent{Path: m.relPath(srcAbs), Operation: "deleted", Timestamp: nowISO()})
	m.publish("file_changed", FileEvent{Path: m.relPath(dstAbs), Operation: "created", Timestamp: nowISO()})

	parsed, err := parser.ParseFile(dstAbs)
	if err != nil {
		return nil, err
	}
	return convertFile(parsed), nil
}

// CopyFile duplicates a hitspec file to a new path within the workspace.
func (m *Manager) CopyFile(ctx context.Context, srcPath, dstPath string) (*ParsedFileDTO, error) {
	_ = ctx
	if err := m.requireWritable(); err != nil {
		return nil, err
	}
	srcAbs, err := m.absPath(srcPath)
	if err != nil {
		return nil, err
	}
	if !isHitspecFile(srcAbs) {
		return nil, fmt.Errorf("only .http and .hitspec files can be copied")
	}
	content, err := os.ReadFile(srcAbs)
	if err != nil {
		return nil, err
	}
	if len(content) > maxRequestBody {
		return nil, fmt.Errorf("file exceeds %d byte limit", maxRequestBody)
	}
	if !isHitspecFile(dstPath) {
		return nil, fmt.Errorf("destination must end in .http or .hitspec")
	}
	dstAbs, err := m.absPath(dstPath)
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(dstAbs); err == nil {
		return nil, fmt.Errorf("destination already exists")
	}
	if err := os.MkdirAll(filepath.Dir(dstAbs), 0o755); err != nil {
		return nil, err
	}
	m.suppressWatch(dstAbs)
	if err := os.WriteFile(dstAbs, content, 0o644); err != nil {
		return nil, err
	}
	m.publish("file_changed", FileEvent{Path: m.relPath(dstAbs), Operation: "created", Timestamp: nowISO()})

	parsed, err := parser.ParseFile(dstAbs)
	if err != nil {
		return nil, err
	}
	return convertFile(parsed), nil
}

func buildFileTree(entries []fileEntry) []FileTreeNodeDTO {
	dirs := map[string]*FileTreeNodeDTO{}
	for _, e := range entries {
		dir := filepath.Dir(e.relPath)
		d := dir
		for d != "." && d != "" {
			if _, ok := dirs[d]; !ok {
				parentDir := filepath.Dir(d)
				if parentDir == "." {
					parentDir = ""
				}
				dirs[d] = &FileTreeNodeDTO{Path: d, Name: filepath.Base(d), Dir: parentDir, IsDir: true}
			}
			d = filepath.Dir(d)
			if d == "." {
				break
			}
		}
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

func sortTreeNodes(nodes []FileTreeNodeDTO) {
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].IsDir != nodes[j].IsDir {
			return nodes[i].IsDir
		}
		return strings.ToLower(nodes[i].Name) < strings.ToLower(nodes[j].Name)
	})
}
