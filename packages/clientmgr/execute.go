package clientmgr

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/abdul-hamid-achik/hitspec/packages/core/runner"
	"github.com/abdul-hamid-achik/hitspec/packages/history"
)

// Execute runs one request from a file.
func (m *Manager) Execute(ctx context.Context, req ExecuteReq) (*RunResultDTO, error) {
	if err := m.requireWritable(); err != nil {
		return nil, err
	}
	if req.File == "" {
		return nil, fmt.Errorf("file is required")
	}
	return m.run(ctx, req.File, req.RequestName, req.Environment)
}

// RunFile runs all requests in a file.
func (m *Manager) RunFile(ctx context.Context, req RunReq) (*RunResultDTO, error) {
	if err := m.requireWritable(); err != nil {
		return nil, err
	}
	if req.File == "" {
		return nil, fmt.Errorf("file is required")
	}
	return m.run(ctx, req.File, "", req.Environment)
}

// ExecuteAdHoc runs a one-off request that is not saved to the workspace. It
// resolves variables from the active (or given) environment just like a normal
// run, by materializing the request into a temporary .http file.
func (m *Manager) ExecuteAdHoc(ctx context.Context, req AdHocReq) (*RunResultDTO, error) {
	_ = ctx
	if err := m.requireWritable(); err != nil {
		return nil, err
	}
	method := strings.ToUpper(strings.TrimSpace(req.Method))
	if method == "" {
		method = "GET"
	}
	if strings.TrimSpace(req.URL) == "" {
		return nil, fmt.Errorf("url is required")
	}
	// The body is round-tripped through the .http parser, which treats a line
	// beginning with ### as a request separator. Reject such bodies so they can't
	// silently truncate the request or spawn a spurious second one.
	for _, line := range strings.Split(req.Body, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "###") {
			return nil, fmt.Errorf("ad-hoc body cannot contain a line starting with ###")
		}
	}

	tmp, err := os.CreateTemp("", "hitspec-adhoc-*.http")
	if err != nil {
		return nil, err
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()
	if _, err := tmp.WriteString(buildAdHocContent(method, req.URL, req.Headers, req.Body)); err != nil {
		_ = tmp.Close()
		return nil, err
	}
	_ = tmp.Close()

	m.configMu.RLock()
	env := m.config.Env
	configEnvs := m.getConfigEnvsLocked()
	m.configMu.RUnlock()
	if req.Environment != "" {
		env = req.Environment
	}

	cfg := &runner.Config{
		Environment:        env,
		Timeout:            30 * time.Second,
		FollowRedirect:     true,
		ValidateSSL:        true,
		ConfigEnvironments: configEnvs,
		AllowShell:         m.config.AllowShell,
		AllowDB:            m.config.AllowDB,
	}
	result, err := runner.NewRunner(cfg).RunFile(tmpPath)
	if err != nil {
		return nil, err
	}
	dto := convertRunResult(result)
	dto.File = "(ad-hoc)"
	m.captureCookies(dto)
	return dto, nil
}

// buildAdHocContent renders an AdHocReq into .http file content with stable
// header ordering.
func buildAdHocContent(method, url string, headers map[string]string, body string) string {
	var sb strings.Builder
	sb.WriteString("### Ad-hoc request\n")
	fmt.Fprintf(&sb, "%s %s\n", method, url)
	keys := make([]string, 0, len(headers))
	for k := range headers {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(&sb, "%s: %s\n", k, headers[k])
	}
	if strings.TrimSpace(body) != "" {
		sb.WriteString("\n")
		sb.WriteString(body)
		sb.WriteString("\n")
	}
	return sb.String()
}

func (m *Manager) run(ctx context.Context, file, requestName, environment string) (*RunResultDTO, error) {
	absPath, err := m.absPath(file)
	if err != nil {
		return nil, err
	}

	m.configMu.RLock()
	env := m.config.Env
	configEnvs := m.getConfigEnvsLocked()
	m.configMu.RUnlock()
	if environment != "" {
		env = environment
	}

	execID := generateID()
	m.publish("execution_start", ExecEvent{ID: execID, File: file, Status: "started", Timestamp: nowISO()})

	cfg := &runner.Config{
		Environment:        env,
		Timeout:            30 * time.Second,
		FollowRedirect:     true,
		ValidateSSL:        true,
		NameFilter:         requestName,
		ConfigEnvironments: configEnvs,
		AllowShell:         m.config.AllowShell,
		AllowDB:            m.config.AllowDB,
	}
	cfg.OnProgress = func(event runner.ProgressEvent) {
		progress := RequestProgress{
			ExecID:      execID,
			File:        file,
			RequestName: event.RequestName,
			Status:      event.Status,
			Index:       event.Index,
			Total:       event.Total,
			Timestamp:   nowISO(),
		}
		if event.Status == "completed" && event.Result != nil {
			progress.Passed = event.Result.Passed
			progress.Duration = float64(event.Result.Duration.Milliseconds())
		}
		m.publish("request_progress", progress)
	}

	rn := runner.NewRunner(cfg)
	result, err := rn.RunFile(absPath)
	if err != nil {
		m.publish("error", ExecEvent{ID: execID, File: file, Status: "error", Error: err.Error(), Timestamp: nowISO()})
		return nil, err
	}

	if requestName != "" {
		executed := make([]*runner.RequestResult, 0, len(result.Results))
		for _, rr := range result.Results {
			if rr.Skipped && rr.SkipReason == "filtered out" {
				continue
			}
			executed = append(executed, rr)
		}
		result.Results = executed
		result.Skipped = 0
		for _, rr := range executed {
			if rr.Skipped {
				result.Skipped++
			}
		}
	}

	dto := convertRunResult(result)
	m.publish("execution_complete", ExecEvent{ID: execID, File: file, Status: "completed", Result: dto, Timestamp: nowISO()})
	for _, rr := range result.Results {
		entry := HistoryEntryDTO{
			ID:          generateID(),
			File:        file,
			RequestName: rr.Name,
			Duration:    float64(rr.Duration.Milliseconds()),
			Passed:      rr.Passed,
			Timestamp:   nowISO(),
		}
		if rr.Request != nil {
			entry.Method = rr.Request.Method
			entry.URL = rr.Request.URL
		}
		if rr.Response != nil {
			entry.StatusCode = rr.Response.StatusCode
		}
		m.history.add(entry)
	}
	m.recordRunToHistory(file, env, dto)
	m.captureCookies(dto)
	return dto, nil
}

// InMemoryHistory returns session-local history entries.
func (m *Manager) InMemoryHistory(ctx context.Context) ([]HistoryEntryDTO, error) {
	_ = ctx
	return m.history.entriesCopy(), nil
}

// ClearInMemoryHistory clears session-local history entries.
func (m *Manager) ClearInMemoryHistory(ctx context.Context) error {
	_ = ctx
	if err := m.requireWritable(); err != nil {
		return err
	}
	m.history.clear()
	return nil
}

// ListRuns lists persistent history runs.
func (m *Manager) ListRuns(ctx context.Context, limit, offset int64) (HistoryListDTO, error) {
	if m.historyStore == nil {
		return HistoryListDTO{}, fmt.Errorf("history database not available")
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	runs, err := m.historyStore.Queries().ListRuns(ctx, history.ListRunsParams{Limit: limit, Offset: offset})
	if err != nil {
		return HistoryListDTO{}, err
	}
	total, err := m.historyStore.Queries().CountRuns(ctx)
	if err != nil {
		return HistoryListDTO{}, err
	}
	dtos := make([]HistoryRunDTO, 0, len(runs))
	for _, run := range runs {
		dtos = append(dtos, convertRunToDTO(run))
	}
	return HistoryListDTO{Runs: dtos, Total: total, Limit: limit, Offset: offset}, nil
}

// GetRun returns one persistent history run with details.
func (m *Manager) GetRun(ctx context.Context, id int64) (HistoryRunDTO, error) {
	if m.historyStore == nil {
		return HistoryRunDTO{}, fmt.Errorf("history database not available")
	}
	run, err := m.historyStore.Queries().GetRun(ctx, id)
	if err != nil {
		return HistoryRunDTO{}, err
	}
	dto := convertRunToDTO(run)
	results, err := m.historyStore.Queries().ListResultsByRun(ctx, id)
	if err != nil {
		return dto, nil
	}
	dto.Results = make([]HistoryResultDTO, 0, len(results))
	for _, res := range results {
		rdto := convertResultToDTO(res)
		assertions, err := m.historyStore.Queries().ListAssertionsByResult(ctx, res.ID)
		if err == nil && len(assertions) > 0 {
			rdto.Assertions = make([]HistoryAssertionDTO, 0, len(assertions))
			for _, a := range assertions {
				rdto.Assertions = append(rdto.Assertions, convertAssertionToDTO(a))
			}
		}
		dto.Results = append(dto.Results, rdto)
	}
	return dto, nil
}

// DeleteRun deletes one persistent run.
func (m *Manager) DeleteRun(ctx context.Context, id int64) error {
	if err := m.requireWritable(); err != nil {
		return err
	}
	if m.historyStore == nil {
		return fmt.Errorf("history database not available")
	}
	return m.historyStore.Queries().DeleteRun(ctx, id)
}

// ClearRuns deletes all persistent runs.
func (m *Manager) ClearRuns(ctx context.Context) error {
	if err := m.requireWritable(); err != nil {
		return err
	}
	if m.historyStore == nil {
		return fmt.Errorf("history database not available")
	}
	return m.historyStore.Queries().ClearAllRuns(ctx)
}

func (m *Manager) recordRunToHistory(filePath, environment string, result *RunResultDTO) {
	if m.historyStore == nil {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		runID, err := m.historyStore.RecordRun(ctx, filePath, environment)
		if err != nil {
			return
		}
		var passed, failed, skipped int64
		for _, rr := range result.Results {
			statusCode := 0
			if rr.Response != nil {
				statusCode = rr.Response.StatusCode
			}
			method, url := "", ""
			if rr.Request != nil {
				method = rr.Request.Method
				url = rr.Request.URL
			}
			bodyPreview := ""
			if rr.Response != nil && len(rr.Response.Body) > 0 {
				bodyPreview = rr.Response.Body
				if len(bodyPreview) > 65536 {
					bodyPreview = bodyPreview[:65536]
				}
			}
			resultID, err := m.historyStore.RecordResult(ctx, runID,
				rr.Name, method, url, statusCode,
				int64(rr.Duration), rr.Passed, rr.Skipped,
				rr.Error, rr.Description, bodyPreview,
			)
			if err != nil {
				continue
			}
			if len(rr.Assertions) > 0 {
				records := make([]history.AssertionRecord, 0, len(rr.Assertions))
				for _, a := range rr.Assertions {
					records = append(records, history.AssertionRecord{
						Operator: a.Operator,
						Subject:  a.Subject,
						Expected: formatAny(a.Expected),
						Actual:   formatAny(a.Actual),
						Passed:   a.Passed,
						Message:  a.Message,
					})
				}
				_ = m.historyStore.RecordAssertions(ctx, resultID, records)
			}
			if rr.Passed {
				passed++
			} else if rr.Skipped {
				skipped++
			} else {
				failed++
			}
		}
		dur := time.Duration(result.Duration) * time.Millisecond
		_ = m.historyStore.FinishRun(ctx, runID, dur, passed, failed, skipped, passed+failed+skipped)
	}()
}

func convertRunToDTO(run history.Run) HistoryRunDTO {
	dto := HistoryRunDTO{
		ID:         run.ID,
		FilePath:   run.FilePath,
		StartedAt:  run.StartedAt.UTC().Format(time.RFC3339),
		DurationMs: run.DurationMs,
		Passed:     run.Passed,
		Failed:     run.Failed,
		Skipped:    run.Skipped,
		Total:      run.Total,
	}
	if run.Environment.Valid {
		dto.Environment = run.Environment.String
	}
	if run.FinishedAt.Valid {
		dto.FinishedAt = run.FinishedAt.Time.UTC().Format(time.RFC3339)
	}
	return dto
}

func convertResultToDTO(res history.Result) HistoryResultDTO {
	dto := HistoryResultDTO{
		ID:          res.ID,
		RequestName: res.RequestName,
		Method:      res.Method,
		URL:         res.Url,
		DurationMs:  res.DurationMs,
		Passed:      res.Passed,
		Skipped:     res.Skipped,
	}
	if res.StatusCode.Valid {
		dto.StatusCode = int(res.StatusCode.Int64)
	}
	if res.Error.Valid {
		dto.Error = res.Error.String
	}
	if res.Description.Valid {
		dto.Description = res.Description.String
	}
	if res.BodyPreview.Valid {
		dto.BodyPreview = res.BodyPreview.String
	}
	return dto
}

func convertAssertionToDTO(a history.Assertion) HistoryAssertionDTO {
	dto := HistoryAssertionDTO{ID: a.ID, Operator: a.Operator, Subject: a.Subject, Passed: a.Passed}
	if a.Expected.Valid {
		dto.Expected = a.Expected.String
	}
	if a.Actual.Valid {
		dto.Actual = a.Actual.String
	}
	if a.Message.Valid {
		dto.Message = a.Message.String
	}
	return dto
}

func formatAny(v any) string {
	if v == nil {
		return ""
	}
	return fmt.Sprintf("%v", v)
}
