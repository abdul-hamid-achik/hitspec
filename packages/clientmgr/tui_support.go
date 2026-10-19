package clientmgr

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/abdul-hamid-achik/hitspec/v2/packages/core/env"
	"github.com/abdul-hamid-achik/hitspec/v2/packages/core/parser"
	"github.com/abdul-hamid-achik/hitspec/v2/packages/core/runner"
	"github.com/abdul-hamid-achik/hitspec/v2/packages/export/snippet"
)

// This file holds the operations the studio TUI needs on top of the file-based
// API: running, parsing and exporting an editor buffer that has not been saved.
// They are additive; the file-based methods are unchanged.

// ParseSource parses unsaved content as if it were the file at relPath, without
// touching the disk. Parse errors are returned so an editor can show them.
func (m *Manager) ParseSource(ctx context.Context, relPath, source string) (*ParsedFileDTO, error) {
	_ = ctx
	absPath, err := m.absPath(relPath)
	if err != nil {
		return nil, err
	}
	parsed, err := parser.Parse(source, absPath)
	if err != nil {
		return nil, err
	}
	return convertFile(parsed), nil
}

// ExecuteSource runs one request of an unsaved buffer. The buffer is parsed in
// place of the file's content on disk, while the file's location is still used
// to resolve environments, body files and snapshots. The request is selected by
// req.RequestName when the buffer has a request with that name, else by
// req.RequestIndex; the caller may set both because the buffer may have been
// edited since the request list was built.
func (m *Manager) ExecuteSource(ctx context.Context, req ExecuteReq, source string) (*RunResultDTO, error) {
	if req.File == "" {
		return nil, fmt.Errorf("file is required")
	}
	absPath, err := m.absPath(req.File)
	if err != nil {
		return nil, err
	}
	parsed, err := parser.Parse(source, absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", req.File, err)
	}
	name, index, err := selectInBuffer(parsed, req.RequestName, req.RequestIndex)
	if err != nil {
		return nil, err
	}
	req.RequestName, req.RequestIndex = name, index
	return m.Execute(runner.WithSource(ctx, source), req)
}

// RunFileSource runs every request of an unsaved buffer (see ExecuteSource).
func (m *Manager) RunFileSource(ctx context.Context, req RunReq, source string) (*RunResultDTO, error) {
	return m.RunFile(runner.WithSource(ctx, source), req)
}

// selectInBuffer resolves which request of a parsed file or buffer a caller means. A
// name that exists in the buffer wins; otherwise the index is used. At least one
// must be given.
func selectInBuffer(f *parser.File, name string, index *int) (string, *int, error) {
	if name != "" {
		for _, r := range f.Requests {
			if r.Name == name {
				return name, nil, nil
			}
		}
	}
	if index != nil {
		if *index < 0 || *index >= len(f.Requests) {
			return "", nil, fmt.Errorf("there is no request %d in this file (it may have changed; re-select a request)", *index+1)
		}
		i := *index
		if n := f.Requests[i].Name; n != "" {
			return n, nil, nil
		}
		return "", &i, nil
	}
	if name != "" {
		return "", nil, fmt.Errorf("there is no request named %q in this file", name)
	}
	return "", nil, fmt.Errorf("no request selected")
}

// ExportSelectionReq asks for client snippets of one request (or every request
// when neither RequestName nor RequestIndex is set) with its {{variables}}
// resolved against the active environment. Source, when non-empty, is an unsaved
// buffer used instead of the file on disk.
type ExportSelectionReq struct {
	File         string
	Source       string
	RequestName  string
	RequestIndex *int
	Format       string
	Environment  string
}

// ExportSelection renders requests as client snippets like Export, but it can
// select an untitled request by position, export an unsaved buffer, and fills in
// {{variables}} from the file, the active environment and the env file, the same
// layers a run uses.
func (m *Manager) ExportSelection(ctx context.Context, req ExportSelectionReq) (ExportResultDTO, error) {
	_ = ctx
	if req.File == "" {
		return ExportResultDTO{}, fmt.Errorf("file is required")
	}
	format, ok := snippet.Normalize(req.Format)
	if !ok {
		return ExportResultDTO{}, fmt.Errorf("unknown export format %q", req.Format)
	}
	absPath, err := m.absPath(req.File)
	if err != nil {
		return ExportResultDTO{}, err
	}
	var parsed *parser.File
	if req.Source != "" {
		parsed, err = parser.Parse(req.Source, absPath)
	} else {
		parsed, err = parser.ParseFileQuiet(absPath)
	}
	if err != nil {
		return ExportResultDTO{}, err
	}

	reqs := parsed.Requests
	if req.RequestName != "" || req.RequestIndex != nil {
		name, index, err := selectInBuffer(parsed, req.RequestName, req.RequestIndex)
		if err != nil {
			return ExportResultDTO{}, err
		}
		reqs = nil
		for i, r := range parsed.Requests {
			if (name != "" && r.Name == name) || (index != nil && i == *index) {
				reqs = append(reqs, r)
				break
			}
		}
	}

	resolve := m.exportResolver(req.Environment, absPath, parsed)
	commands := make([]string, 0, len(reqs))
	rendered := 0
	var lastErr error
	for _, r := range reqs {
		cmd, err := snippet.Render(format, r, snippet.WithResolver(resolve))
		if err != nil {
			if len(reqs) == 1 {
				return ExportResultDTO{}, err
			}
			lastErr = err
			cmd = snippet.Comment(format) + " SKIPPED: " + err.Error()
		} else {
			rendered++
		}
		commands = append(commands, cmd)
	}
	if rendered == 0 && lastErr != nil {
		return ExportResultDTO{}, lastErr
	}
	return ExportResultDTO{Commands: commands}, nil
}

// exportResolver builds the {{variable}} resolver an export uses: the active
// environment (config environments and the env file), then the file's own
// variables on top, the same precedence a run applies.
func (m *Manager) exportResolver(environment, absPath string, f *parser.File) func(string) string {
	cfg, _ := m.runSettings(environment)
	resolver := env.NewResolver()
	if cfg.EnvFile != "" {
		_ = resolver.LoadDotEnv(cfg.EnvFile)
	}
	if e, err := env.LoadEnvironment(filepath.Dir(absPath), cfg.Environment, cfg.ConfigEnvironments); err == nil {
		resolver.SetVariables(e.Variables)
	}
	for _, v := range f.Variables {
		resolver.SetVariable(v.Name, v.Value)
	}
	return resolver.Resolve
}

// CookieStorePath returns the file that backs the cookie store. The store lives
// in the user's home directory and is shared by every workspace.
func (m *Manager) CookieStorePath() string {
	p, err := cookieStorePath()
	if err != nil {
		return ""
	}
	return p
}
