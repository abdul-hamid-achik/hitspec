package clientmgr

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/abdul-hamid-achik/hitspec/packages/contract"
	"github.com/abdul-hamid-achik/hitspec/packages/core/config"
	"github.com/abdul-hamid-achik/hitspec/packages/core/parser"
	curlexport "github.com/abdul-hamid-achik/hitspec/packages/export/curl"
	curlimport "github.com/abdul-hamid-achik/hitspec/packages/import/curl"
	"github.com/abdul-hamid-achik/hitspec/packages/import/insomnia"
	"github.com/abdul-hamid-achik/hitspec/packages/import/openapi"
	"github.com/abdul-hamid-achik/hitspec/packages/mock"
	"github.com/abdul-hamid-achik/hitspec/packages/proxy"
	"github.com/abdul-hamid-achik/hitspec/packages/stress"
)

// StartStress starts a background stress test.
func (m *Manager) StartStress(ctx context.Context, req StressStartReq) error {
	if err := m.requireWritable(); err != nil {
		return err
	}
	m.mu.Lock()
	if m.stressRunner != nil {
		m.mu.Unlock()
		return fmt.Errorf("stress test already running")
	}
	duration, err := time.ParseDuration(req.Duration)
	if err != nil {
		m.mu.Unlock()
		return fmt.Errorf("invalid duration: %w", err)
	}
	cfg := stress.DefaultConfig()
	cfg.Duration = duration
	if req.Rate > 0 {
		cfg.Rate = req.Rate
	}
	if req.VUs > 0 {
		cfg.VUs = req.VUs
		cfg.Mode = stress.VUMode
	}
	if req.MaxVUs > 0 {
		cfg.MaxVUs = req.MaxVUs
	}
	absFiles := make([]string, 0, len(req.Files))
	for _, f := range req.Files {
		absPath, err := m.absPath(f)
		if err != nil {
			m.mu.Unlock()
			return err
		}
		absFiles = append(absFiles, absPath)
	}
	stressRunner := stress.NewRunner(cfg)
	if err := stressRunner.LoadFiles(absFiles); err != nil {
		m.mu.Unlock()
		return err
	}
	baseCtx := m.ctx
	if baseCtx == nil {
		baseCtx = ctx
	}
	runCtx, cancel := context.WithCancel(baseCtx)
	m.stressRunner = stressRunner
	m.stressCancel = cancel
	m.mu.Unlock()

	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		done := make(chan struct{})
		go func() {
			defer close(done)
			_, _ = stressRunner.Run(runCtx)
		}()
		for {
			select {
			case <-done:
				stats := stressRunner.GetCurrentStats()
				resultDTO := convertStressResult(stressRunner)
				m.publish("stress_update", StressMetrics{
					Running:   false,
					Completed: true,
					Stats:     convertStressStats(stats),
					Elapsed:   stats.Elapsed.Seconds(),
					Timestamp: nowISO(),
				})
				m.mu.Lock()
				m.lastStressResult = resultDTO
				m.stressRunner = nil
				m.stressCancel = nil
				m.mu.Unlock()
				return
			case <-ticker.C:
				stats := stressRunner.GetCurrentStats()
				m.publish("stress_update", StressMetrics{
					Running:   true,
					Stats:     convertStressStats(stats),
					Elapsed:   stats.Elapsed.Seconds(),
					Timestamp: nowISO(),
				})
			}
		}
	}()
	return nil
}

// StopStress stops a stress test.
func (m *Manager) StopStress(ctx context.Context) error {
	_ = ctx
	if err := m.requireWritable(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.stressCancel != nil {
		m.stressCancel()
	}
	return nil
}

// StressStatus returns the current stress status.
func (m *Manager) StressStatus(ctx context.Context) StressStatusDTO {
	_ = ctx
	m.mu.Lock()
	runner := m.stressRunner
	m.mu.Unlock()
	if runner == nil {
		return StressStatusDTO{Running: false}
	}
	stats := runner.GetCurrentStats()
	dto := convertStressStats(stats)
	return StressStatusDTO{Running: true, Elapsed: stats.Elapsed.Seconds(), Stats: &dto}
}

// StressResult returns the last completed stress result.
func (m *Manager) StressResult(ctx context.Context) (*StressResultDTO, error) {
	_ = ctx
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.lastStressResult == nil {
		return nil, fmt.Errorf("no stress test result available")
	}
	return m.lastStressResult, nil
}

// ListStressProfiles returns configured stress profiles.
func (m *Manager) ListStressProfiles(ctx context.Context) ([]StressProfileDTO, error) {
	_ = ctx
	m.configMu.RLock()
	defer m.configMu.RUnlock()
	var profiles []StressProfileDTO
	if m.fileConfig != nil && m.fileConfig.Stress != nil && m.fileConfig.Stress.Profiles != nil {
		for name, p := range m.fileConfig.Stress.Profiles {
			profiles = append(profiles, StressProfileDTO{
				Name:       name,
				Duration:   p.Duration,
				Rate:       p.Rate,
				VUs:        p.VUs,
				MaxVUs:     p.MaxVUs,
				ThinkTime:  p.ThinkTime,
				RampUp:     p.RampUp,
				Thresholds: p.Thresholds,
			})
		}
	}
	if profiles == nil {
		profiles = []StressProfileDTO{}
	}
	return profiles, nil
}

// PutStressProfile creates or updates a stress profile.
func (m *Manager) PutStressProfile(ctx context.Context, req StressProfileReq) error {
	_ = ctx
	if err := m.requireWritable(); err != nil {
		return err
	}
	if req.Name == "" {
		return fmt.Errorf("profile name is required")
	}
	m.configMu.Lock()
	defer m.configMu.Unlock()
	if m.fileConfig == nil {
		m.fileConfig = config.DefaultConfig()
	}
	if m.fileConfig.Stress == nil {
		m.fileConfig.Stress = &config.StressConfig{}
	}
	if m.fileConfig.Stress.Profiles == nil {
		m.fileConfig.Stress.Profiles = make(map[string]*config.StressProfile)
	}
	m.fileConfig.Stress.Profiles[req.Name] = &config.StressProfile{
		Duration:   req.Duration,
		Rate:       req.Rate,
		VUs:        req.VUs,
		MaxVUs:     req.MaxVUs,
		ThinkTime:  req.ThinkTime,
		RampUp:     req.RampUp,
		Thresholds: req.Thresholds,
	}
	return m.saveConfigLocked()
}

// DeleteStressProfile removes a stress profile.
func (m *Manager) DeleteStressProfile(ctx context.Context, name string) error {
	_ = ctx
	if err := m.requireWritable(); err != nil {
		return err
	}
	m.configMu.Lock()
	defer m.configMu.Unlock()
	if m.fileConfig == nil || m.fileConfig.Stress == nil || m.fileConfig.Stress.Profiles == nil {
		return fmt.Errorf("profile not found: %s", name)
	}
	delete(m.fileConfig.Stress.Profiles, name)
	return m.saveConfigLocked()
}

// StartMock starts a mock server from files.
func (m *Manager) StartMock(ctx context.Context, req MockStartReq) (MockStatusDTO, error) {
	if err := m.requireWritable(); err != nil {
		return MockStatusDTO{}, err
	}
	m.mu.Lock()
	if m.mockServer != nil {
		m.mu.Unlock()
		return MockStatusDTO{}, fmt.Errorf("mock server already running")
	}
	port := req.Port
	if port == 0 {
		port = 3000
	}
	var delay time.Duration
	if req.Delay != "" {
		var err error
		delay, err = time.ParseDuration(req.Delay)
		if err != nil {
			m.mu.Unlock()
			return MockStatusDTO{}, err
		}
	}
	absFiles := make([]string, 0, len(req.Files))
	for _, f := range req.Files {
		absPath, err := m.absPath(f)
		if err != nil {
			m.mu.Unlock()
			return MockStatusDTO{}, err
		}
		absFiles = append(absFiles, absPath)
	}
	mockSrv := mock.NewServer(
		mock.WithPort(port),
		mock.WithDelay(delay),
		mock.WithVerbose(m.config.Verbose),
		mock.WithRequestCallback(func(method, path string, status int, duration time.Duration) {
			m.publish("mock_request", MockEvent{
				Event:     "request",
				Method:    method,
				Path:      path,
				Status:    status,
				Duration:  float64(duration.Milliseconds()),
				Timestamp: nowISO(),
			})
		}),
	)
	if err := mockSrv.LoadFiles(absFiles); err != nil {
		m.mu.Unlock()
		return MockStatusDTO{}, err
	}
	baseCtx := m.ctx
	if baseCtx == nil {
		baseCtx = ctx
	}
	runCtx, cancel := context.WithCancel(baseCtx)
	m.mockServer = mockSrv
	m.mockCancel = cancel
	m.mockPort = port
	m.mu.Unlock()

	go func() {
		m.publish("mock_request", MockEvent{Event: "started", Timestamp: nowISO()})
		_ = mockSrv.StartWithContext(runCtx)
		m.publish("mock_request", MockEvent{Event: "stopped", Timestamp: nowISO()})
		m.mu.Lock()
		m.mockServer = nil
		m.mockCancel = nil
		m.mockPort = 0
		m.mu.Unlock()
	}()
	return m.MockStatus(ctx), nil
}

// StopMock stops the running mock server.
func (m *Manager) StopMock(ctx context.Context) error {
	_ = ctx
	if err := m.requireWritable(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.mockCancel == nil {
		return fmt.Errorf("no mock server running")
	}
	m.mockCancel()
	return nil
}

// MockStatus returns mock server status and routes.
func (m *Manager) MockStatus(ctx context.Context) MockStatusDTO {
	_ = ctx
	m.mu.Lock()
	mockSrv := m.mockServer
	port := m.mockPort
	m.mu.Unlock()
	if mockSrv == nil {
		return MockStatusDTO{Running: false}
	}
	routes := mockSrv.GetRoutes()
	routeDTOs := make([]MockRouteDTO, 0, len(routes))
	for _, route := range routes {
		routeDTOs = append(routeDTOs, MockRouteDTO{
			Method:      route.Method,
			Path:        route.PathPattern,
			Name:        route.Name,
			StatusCode:  route.Response.StatusCode,
			ContentType: route.Response.ContentType,
		})
	}
	return MockStatusDTO{Running: true, Port: port, Routes: routeDTOs}
}

// StartRecord starts the recording proxy.
func (m *Manager) StartRecord(ctx context.Context, req RecordStartReq) error {
	if err := m.requireWritable(); err != nil {
		return err
	}
	if req.TargetURL == "" {
		return fmt.Errorf("targetUrl is required")
	}
	m.mu.Lock()
	if m.recorderRunning {
		m.mu.Unlock()
		return fmt.Errorf("recording proxy already running")
	}
	port := req.Port
	if port == 0 {
		port = 8081
	}
	opts := []proxy.Option{
		proxy.WithTargetURL(req.TargetURL),
		proxy.WithPort(port),
		proxy.WithVerbose(m.config.Verbose),
		proxy.WithDeduplicate(req.Deduplicate),
	}
	if len(req.Exclude) > 0 {
		opts = append(opts, proxy.WithExclude(req.Exclude))
	}
	if len(req.Sanitize) > 0 {
		opts = append(opts, proxy.WithSanitize(req.Sanitize))
	}
	recorder := proxy.NewRecorder(opts...)
	baseCtx := m.ctx
	if baseCtx == nil {
		baseCtx = ctx
	}
	runCtx, cancel := context.WithCancel(baseCtx)
	m.recorder = recorder
	m.recorderCancel = cancel
	m.recorderRunning = true
	m.recorderPort = port
	m.recorderTarget = req.TargetURL
	m.mu.Unlock()
	go func() {
		_ = recorder.StartWithContext(runCtx)
		m.mu.Lock()
		// Keep the recorder reference so recordings remain available for
		// export/status after the proxy stops; only ClearRecordings drops
		// them. Track the live state separately from the recorder instance.
		m.recorderRunning = false
		m.recorderCancel = nil
		m.mu.Unlock()
	}()
	return nil
}

// StopRecord stops the recording proxy.
func (m *Manager) StopRecord(ctx context.Context) error {
	_ = ctx
	if err := m.requireWritable(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.recorderCancel == nil {
		return fmt.Errorf("no recording proxy running")
	}
	m.recorderCancel()
	return nil
}

// RecordStatus returns recorder status.
func (m *Manager) RecordStatus(ctx context.Context) RecordStatusDTO {
	_ = ctx
	m.mu.Lock()
	recorder := m.recorder
	running := m.recorderRunning
	port := m.recorderPort
	target := m.recorderTarget
	m.mu.Unlock()
	if recorder == nil {
		return RecordStatusDTO{Running: false}
	}
	recordings := recorder.GetRecordings()
	dtos := make([]RecordingDTO, 0, len(recordings))
	for _, rec := range recordings {
		dto := RecordingDTO{
			Method:      rec.Method,
			Path:        rec.Path,
			URL:         rec.URL,
			ContentType: rec.ContentType,
		}
		if rec.Response != nil {
			dto.StatusCode = rec.Response.StatusCode
			dto.Duration = float64(rec.Response.Duration.Milliseconds())
		}
		dtos = append(dtos, dto)
	}
	return RecordStatusDTO{Running: running, TargetURL: target, Port: port, Count: len(recordings), Recordings: dtos}
}

// ExportRecordings returns recorded requests as hitspec content.
func (m *Manager) ExportRecordings(ctx context.Context) (string, error) {
	_ = ctx
	m.mu.Lock()
	recorder := m.recorder
	m.mu.Unlock()
	if recorder == nil {
		return "", fmt.Errorf("no recordings available")
	}
	return recorder.Export(), nil
}

// ClearRecordings clears captured requests.
func (m *Manager) ClearRecordings(ctx context.Context) error {
	_ = ctx
	if err := m.requireWritable(); err != nil {
		return err
	}
	m.mu.Lock()
	recorder := m.recorder
	m.mu.Unlock()
	if recorder == nil {
		return fmt.Errorf("no recordings to clear")
	}
	recorder.Clear()
	return nil
}

// VerifyContracts verifies contract files against a provider URL.
func (m *Manager) VerifyContracts(ctx context.Context, req ContractVerifyReq) ([]ContractResultDTO, error) {
	_ = ctx
	if err := m.requireWritable(); err != nil {
		return nil, err
	}
	if req.ProviderURL == "" {
		return nil, fmt.Errorf("providerUrl is required")
	}
	opts := []contract.Option{
		contract.WithProviderURL(req.ProviderURL),
		contract.WithVerbose(m.config.Verbose),
	}
	if req.StateHandler != "" {
		opts = append(opts, contract.WithStateHandler(req.StateHandler))
	}
	verifier := contract.NewVerifier(opts...)
	files := req.Files
	if len(files) == 0 {
		collected, err := collectHitspecFiles(m.config.WorkDir)
		if err != nil {
			return nil, err
		}
		files = collected
	}
	var results []ContractResultDTO
	for _, file := range files {
		absPath := file
		if !filepath.IsAbs(file) {
			var err error
			absPath, err = m.absPath(file)
			if err != nil {
				continue
			}
		}
		if !isPathWithin(m.config.WorkDir, absPath) {
			continue
		}
		result, err := verifier.VerifyFile(absPath)
		if err != nil {
			results = append(results, ContractResultDTO{
				File: file,
				Results: []ContractInteractionDTO{{
					Name:   "parse",
					Passed: false,
					Error:  err.Error(),
				}},
			})
			continue
		}
		dto := ContractResultDTO{
			File:     file,
			Passed:   result.Passed,
			Failed:   result.Failed,
			Skipped:  result.Skipped,
			Duration: float64(result.Duration) / float64(time.Millisecond),
		}
		for _, ir := range result.Results {
			interaction := ContractInteractionDTO{
				Name:     ir.Name,
				Provider: ir.Provider,
				State:    ir.State,
				Passed:   ir.Passed,
				Duration: float64(ir.Duration) / float64(time.Millisecond),
			}
			if ir.Error != nil {
				interaction.Error = ir.Error.Error()
			}
			dto.Results = append(dto.Results, interaction)
		}
		results = append(results, dto)
	}
	return results, nil
}

// ContractFiles lists candidate contract files.
func (m *Manager) ContractFiles(ctx context.Context) (ContractStatusDTO, error) {
	_ = ctx
	files, err := collectHitspecFiles(m.config.WorkDir)
	if err != nil {
		return ContractStatusDTO{}, err
	}
	relFiles := make([]string, 0, len(files))
	for _, f := range files {
		relFiles = append(relFiles, m.relPath(f))
	}
	return ContractStatusDTO{Files: relFiles}, nil
}

// ImportCurl imports from curl.
func (m *Manager) ImportCurl(ctx context.Context, req ImportCurlReq) (ImportResultDTO, error) {
	_ = ctx
	converter := curlimport.NewConverter()
	var content string
	var err error
	if req.Command != "" {
		content, err = converter.ConvertCommand(req.Command)
	} else if req.FilePath != "" {
		absPath, e := m.absPath(req.FilePath)
		if e != nil {
			return ImportResultDTO{}, e
		}
		content, err = converter.ConvertFile(absPath)
	} else {
		return ImportResultDTO{}, fmt.Errorf("command or filePath is required")
	}
	if err != nil {
		return ImportResultDTO{}, err
	}
	return ImportResultDTO{Content: content, RequestCount: countRequests(content)}, nil
}

// ImportInsomnia imports from Insomnia.
func (m *Manager) ImportInsomnia(ctx context.Context, req ImportInsomniaReq) (ImportResultDTO, error) {
	_ = ctx
	converter := insomnia.NewConverter()
	var content string
	var err error
	if req.Data != "" {
		content, err = converter.Convert([]byte(req.Data))
	} else if req.FilePath != "" {
		absPath, e := m.absPath(req.FilePath)
		if e != nil {
			return ImportResultDTO{}, e
		}
		content, err = converter.ConvertFile(absPath)
	} else {
		return ImportResultDTO{}, fmt.Errorf("data or filePath is required")
	}
	if err != nil {
		return ImportResultDTO{}, err
	}
	return ImportResultDTO{Content: content, RequestCount: countRequests(content)}, nil
}

// ImportOpenAPI imports from OpenAPI.
func (m *Manager) ImportOpenAPI(ctx context.Context, req ImportOpenAPIReq) (ImportResultDTO, error) {
	_ = ctx
	if req.SpecPath == "" {
		return ImportResultDTO{}, fmt.Errorf("specPath is required")
	}
	opts := []openapi.Option{}
	if req.BaseURL != "" {
		opts = append(opts, openapi.WithBaseURL(req.BaseURL))
	}
	converter := openapi.NewConverter(opts...)
	specPath := req.SpecPath
	if strings.HasPrefix(specPath, "http://") || strings.HasPrefix(specPath, "https://") {
		u, err := url.Parse(specPath)
		if err != nil {
			return ImportResultDTO{}, err
		}
		host := strings.ToLower(u.Hostname())
		if host == "localhost" || host == "127.0.0.1" || host == "::1" ||
			host == "0.0.0.0" || strings.HasPrefix(host, "10.") ||
			strings.HasPrefix(host, "192.168.") || strings.HasPrefix(host, "172.") ||
			host == "169.254.169.254" || host == "metadata.google.internal" {
			return ImportResultDTO{}, fmt.Errorf("URLs pointing to internal/private addresses are not allowed")
		}
	} else {
		var err error
		specPath, err = m.absPath(specPath)
		if err != nil {
			return ImportResultDTO{}, err
		}
	}
	content, err := converter.ConvertFile(specPath)
	if err != nil {
		return ImportResultDTO{}, err
	}
	return ImportResultDTO{Content: content, RequestCount: countRequests(content)}, nil
}

// ImportPostman imports from a Postman Collection v2.1 JSON document.
func (m *Manager) ImportPostman(ctx context.Context, req ImportPostmanReq) (ImportResultDTO, error) {
	_ = ctx
	data := []byte(req.Data)
	if req.FilePath != "" {
		absPath, err := m.absPath(req.FilePath)
		if err != nil {
			return ImportResultDTO{}, err
		}
		fileData, err := os.ReadFile(absPath)
		if err != nil {
			return ImportResultDTO{}, err
		}
		data = fileData
	}
	if len(data) == 0 {
		return ImportResultDTO{}, fmt.Errorf("data or filePath is required")
	}
	content, err := convertPostmanData(data)
	if err != nil {
		return ImportResultDTO{}, err
	}
	return ImportResultDTO{Content: content, RequestCount: countRequests(content)}, nil
}

// Export exports requests to curl or a compact text snippet for other client formats.
func (m *Manager) Export(ctx context.Context, req ExportReq) (ExportResultDTO, error) {
	_ = ctx
	if req.File == "" {
		return ExportResultDTO{}, fmt.Errorf("file is required")
	}
	absPath, err := m.absPath(req.File)
	if err != nil {
		return ExportResultDTO{}, err
	}
	parsed, err := parser.ParseFile(absPath)
	if err != nil {
		return ExportResultDTO{}, err
	}
	var reqs []*parser.Request
	if req.RequestName != "" {
		for _, r := range parsed.Requests {
			if r.Name == req.RequestName {
				reqs = append(reqs, r)
				break
			}
		}
		if len(reqs) == 0 {
			return ExportResultDTO{}, fmt.Errorf("request not found: %s", req.RequestName)
		}
	} else {
		reqs = parsed.Requests
	}
	format := strings.ToLower(req.Format)
	if format == "" || format == "curl" {
		exporter := curlexport.New()
		return ExportResultDTO{Commands: exporter.ExportAll(reqs)}, nil
	}
	commands := make([]string, 0, len(reqs))
	for _, r := range reqs {
		commands = append(commands, exportSnippet(format, r))
	}
	return ExportResultDTO{Commands: commands}, nil
}

func convertStressResult(runner *stress.Runner) *StressResultDTO {
	summary := runner.GetSummary()
	if summary == nil {
		return nil
	}
	dto := &StressResultDTO{
		DurationMs:  float64(summary.Duration.Milliseconds()),
		Total:       summary.TotalRequests,
		Success:     summary.SuccessCount,
		Errors:      summary.ErrorCount,
		Timeouts:    summary.TimeoutCount,
		RPS:         summary.RPS,
		SuccessRate: summary.SuccessRate,
		ErrorRate:   summary.ErrorRate,
		P50Ms:       float64(summary.P50.Microseconds()) / 1000.0,
		P95Ms:       float64(summary.P95.Microseconds()) / 1000.0,
		P99Ms:       float64(summary.P99.Microseconds()) / 1000.0,
		MinMs:       float64(summary.Min.Microseconds()) / 1000.0,
		MaxMs:       float64(summary.Max.Microseconds()) / 1000.0,
		MeanMs:      float64(summary.Mean.Microseconds()) / 1000.0,
		StdDevMs:    float64(summary.StdDev.Microseconds()) / 1000.0,
		Timestamp:   nowISO(),
	}
	for _, rs := range summary.RequestBreakdown {
		dto.Breakdown = append(dto.Breakdown, StressRequestBreakdownDTO{
			Name:    rs.Name,
			Total:   rs.Total,
			Success: rs.Success,
			Errors:  rs.Errors,
			P50Ms:   float64(rs.P50.Microseconds()) / 1000.0,
			P95Ms:   float64(rs.P95.Microseconds()) / 1000.0,
			P99Ms:   float64(rs.P99.Microseconds()) / 1000.0,
			MeanMs:  float64(rs.Mean.Microseconds()) / 1000.0,
		})
	}
	if dto.Breakdown == nil {
		dto.Breakdown = []StressRequestBreakdownDTO{}
	}
	for _, tp := range summary.TimeSeries {
		dto.TimeSeries = append(dto.TimeSeries, StressTimePointDTO{
			Timestamp: tp.Timestamp.UTC().Format("2006-01-02T15:04:05.000Z"),
			Requests:  tp.Requests,
			Errors:    tp.Errors,
			P50Ms:     float64(tp.P50.Microseconds()) / 1000.0,
			P95Ms:     float64(tp.P95.Microseconds()) / 1000.0,
			P99Ms:     float64(tp.P99.Microseconds()) / 1000.0,
			RPS:       tp.RPS,
			ActiveVUs: tp.ActiveVUs,
		})
	}
	if dto.TimeSeries == nil {
		dto.TimeSeries = []StressTimePointDTO{}
	}
	return dto
}

func convertStressStats(s stress.CurrentStats) StressStatsDTO {
	return StressStatsDTO{
		Total:     s.Total,
		Success:   s.Success,
		Errors:    s.Errors,
		RPS:       s.RPS,
		P50Ms:     float64(s.P50.Microseconds()) / 1000.0,
		P95Ms:     float64(s.P95.Microseconds()) / 1000.0,
		P99Ms:     float64(s.P99.Microseconds()) / 1000.0,
		MaxMs:     float64(s.Max.Microseconds()) / 1000.0,
		ErrorRate: s.ErrorRate,
		ActiveVUs: s.ActiveVUs,
	}
}

// exportSnippet renders a request as a runnable snippet in the given language,
// including method, URL, headers, and body. (curl has its own dedicated exporter
// in packages/export/curl; this covers the other languages.)
func exportSnippet(format string, r *parser.Request) string {
	method := r.Method
	if method == "" {
		method = "GET"
	}
	switch format {
	case "fetch", "javascript":
		return fetchSnippet(method, r)
	case "wget":
		return wgetSnippet(method, r)
	case "python":
		return pythonSnippet(method, r)
	case "httpie":
		return httpieSnippet(method, r)
	case "go":
		return goSnippet(method, r)
	case "ruby":
		return rubySnippet(method, r)
	default:
		return fmt.Sprintf("%s %s", method, r.URL)
	}
}

// snippetBody returns the trimmed raw request body, or "" when there is none.
func snippetBody(r *parser.Request) string {
	if r.Body == nil || r.Body.ContentType == parser.BodyNone {
		return ""
	}
	return strings.TrimSpace(r.Body.Raw)
}

// shellQuote wraps s in single quotes, safely escaping embedded single quotes,
// for shell-based snippets (httpie, wget).
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func fetchSnippet(method string, r *parser.Request) string {
	var b strings.Builder
	fmt.Fprintf(&b, "fetch(%s, {\n  method: %s", strconv.Quote(r.URL), strconv.Quote(method))
	if len(r.Headers) > 0 {
		b.WriteString(",\n  headers: {")
		for i, h := range r.Headers {
			if i > 0 {
				b.WriteString(",")
			}
			fmt.Fprintf(&b, "\n    %s: %s", strconv.Quote(h.Key), strconv.Quote(h.Value))
		}
		b.WriteString("\n  }")
	}
	if body := snippetBody(r); body != "" {
		fmt.Fprintf(&b, ",\n  body: %s", strconv.Quote(body))
	}
	b.WriteString("\n})")
	return b.String()
}

func pythonSnippet(method string, r *parser.Request) string {
	var b strings.Builder
	b.WriteString("import requests\n\n")
	if len(r.Headers) > 0 {
		b.WriteString("headers = {\n")
		for _, h := range r.Headers {
			fmt.Fprintf(&b, "    %s: %s,\n", strconv.Quote(h.Key), strconv.Quote(h.Value))
		}
		b.WriteString("}\n")
	}
	body := snippetBody(r)
	if body != "" {
		fmt.Fprintf(&b, "data = %s\n", strconv.Quote(body))
	}
	fmt.Fprintf(&b, "response = requests.request(%s, %s", strconv.Quote(method), strconv.Quote(r.URL))
	if len(r.Headers) > 0 {
		b.WriteString(", headers=headers")
	}
	if body != "" {
		b.WriteString(", data=data")
	}
	b.WriteString(")")
	return b.String()
}

func httpieSnippet(method string, r *parser.Request) string {
	var b strings.Builder
	if body := snippetBody(r); body != "" {
		fmt.Fprintf(&b, "echo %s | ", shellQuote(body))
	}
	fmt.Fprintf(&b, "http %s %s", method, shellQuote(r.URL))
	for _, h := range r.Headers {
		fmt.Fprintf(&b, " %s", shellQuote(h.Key+":"+h.Value))
	}
	return b.String()
}

func goSnippet(method string, r *parser.Request) string {
	var b strings.Builder
	if body := snippetBody(r); body != "" {
		bodyLit := "`" + body + "`"
		if strings.Contains(body, "`") {
			bodyLit = strconv.Quote(body)
		}
		fmt.Fprintf(&b, "req, _ := http.NewRequest(%s, %s, strings.NewReader(%s))\n", strconv.Quote(method), strconv.Quote(r.URL), bodyLit)
	} else {
		fmt.Fprintf(&b, "req, _ := http.NewRequest(%s, %s, nil)\n", strconv.Quote(method), strconv.Quote(r.URL))
	}
	for _, h := range r.Headers {
		fmt.Fprintf(&b, "req.Header.Set(%s, %s)\n", strconv.Quote(h.Key), strconv.Quote(h.Value))
	}
	b.WriteString("resp, _ := http.DefaultClient.Do(req)\ndefer resp.Body.Close()")
	return b.String()
}

func rubySnippet(method string, r *parser.Request) string {
	var b strings.Builder
	b.WriteString("require 'net/http'\nrequire 'uri'\n\n")
	fmt.Fprintf(&b, "uri = URI(%s)\n", strconv.Quote(r.URL))
	fmt.Fprintf(&b, "req = Net::HTTP::%s.new(uri)\n", rubyMethodClass(method))
	for _, h := range r.Headers {
		fmt.Fprintf(&b, "req[%s] = %s\n", strconv.Quote(h.Key), strconv.Quote(h.Value))
	}
	if body := snippetBody(r); body != "" {
		fmt.Fprintf(&b, "req.body = %s\n", strconv.Quote(body))
	}
	b.WriteString("res = Net::HTTP.start(uri.hostname, uri.port, use_ssl: uri.scheme == 'https') { |http| http.request(req) }")
	return b.String()
}

func wgetSnippet(method string, r *parser.Request) string {
	var b strings.Builder
	fmt.Fprintf(&b, "wget --method=%s", method)
	for _, h := range r.Headers {
		fmt.Fprintf(&b, " --header=%s", shellQuote(h.Key+": "+h.Value))
	}
	if body := snippetBody(r); body != "" {
		fmt.Fprintf(&b, " --body-data=%s", shellQuote(body))
	}
	fmt.Fprintf(&b, " %s", shellQuote(r.URL))
	return b.String()
}

// rubyMethodClass title-cases an HTTP method for Ruby's Net::HTTP class names
// (get -> Get, post -> Post).
func rubyMethodClass(method string) string {
	m := strings.ToLower(method)
	if m == "" {
		return m
	}
	return strings.ToUpper(m[:1]) + m[1:]
}

type postmanCollection struct {
	Info postmanInfo   `json:"info"`
	Item []postmanItem `json:"item"`
}

type postmanInfo struct {
	Name string `json:"name"`
}

type postmanItem struct {
	Name    string        `json:"name"`
	Request *postmanReq   `json:"request,omitempty"`
	Item    []postmanItem `json:"item,omitempty"`
}

type postmanReq struct {
	Method string          `json:"method"`
	Header []postmanHeader `json:"header,omitempty"`
	Body   *postmanBody    `json:"body,omitempty"`
	URL    postmanURL      `json:"url"`
}

type postmanHeader struct {
	Key      string `json:"key"`
	Value    string `json:"value"`
	Disabled bool   `json:"disabled,omitempty"`
}

type postmanBody struct {
	Mode       string            `json:"mode"`
	Raw        string            `json:"raw,omitempty"`
	URLEncoded []postmanKV       `json:"urlencoded,omitempty"`
	FormData   []postmanFormData `json:"formdata,omitempty"`
}

type postmanKV struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type postmanFormData struct {
	Key   string `json:"key"`
	Value string `json:"value"`
	Type  string `json:"type"`
	Src   string `json:"src,omitempty"`
}

type postmanURL struct {
	Raw string `json:"raw"`
}

func convertPostmanData(data []byte) (string, error) {
	var collection postmanCollection
	if err := json.Unmarshal(data, &collection); err != nil {
		return "", fmt.Errorf("failed to parse Postman collection: %w", err)
	}
	var sb strings.Builder
	sb.WriteString("# Generated from Postman collection")
	if collection.Info.Name != "" {
		sb.WriteString(": ")
		sb.WriteString(collection.Info.Name)
	}
	sb.WriteString("\n\n")
	convertPostmanItems(&sb, collection.Item, "")
	return sb.String(), nil
}

func convertPostmanItems(sb *strings.Builder, items []postmanItem, prefix string) {
	for _, item := range items {
		if len(item.Item) > 0 {
			nextPrefix := item.Name
			if prefix != "" {
				nextPrefix = prefix + "/" + item.Name
			}
			convertPostmanItems(sb, item.Item, nextPrefix)
			continue
		}
		if item.Request == nil {
			continue
		}
		sb.WriteString("### ")
		if prefix != "" {
			sb.WriteString(prefix)
			sb.WriteString(" - ")
		}
		sb.WriteString(item.Name)
		sb.WriteString("\n# @name ")
		sb.WriteString(sanitizePostmanName(item.Name))
		sb.WriteString("\n")

		req := item.Request
		sb.WriteString(req.Method)
		sb.WriteString(" ")
		sb.WriteString(convertPostmanVariable(req.URL.Raw))
		sb.WriteString("\n")
		for _, h := range req.Header {
			if h.Disabled {
				continue
			}
			sb.WriteString(h.Key)
			sb.WriteString(": ")
			sb.WriteString(convertPostmanVariable(h.Value))
			sb.WriteString("\n")
		}
		if req.Body != nil {
			switch req.Body.Mode {
			case "raw":
				if req.Body.Raw != "" {
					sb.WriteString("\n")
					sb.WriteString(convertPostmanVariable(req.Body.Raw))
					sb.WriteString("\n")
				}
			case "urlencoded":
				if len(req.Body.URLEncoded) > 0 {
					sb.WriteString("\n")
					var parts []string
					for _, kv := range req.Body.URLEncoded {
						parts = append(parts, kv.Key+"="+convertPostmanVariable(kv.Value))
					}
					sb.WriteString(strings.Join(parts, "&"))
					sb.WriteString("\n")
				}
			case "formdata":
				if len(req.Body.FormData) > 0 {
					sb.WriteString("\n")
					for _, kv := range req.Body.FormData {
						value := kv.Value
						if kv.Type == "file" && kv.Src != "" {
							value = "@" + kv.Src
						}
						sb.WriteString(kv.Key)
						sb.WriteString("=")
						sb.WriteString(convertPostmanVariable(value))
						sb.WriteString("\n")
					}
				}
			}
		}
		sb.WriteString("\n>>>\nexpect status == 200\n<<<\n\n")
	}
}

func convertPostmanVariable(s string) string {
	s = strings.ReplaceAll(s, "{{$guid}}", "{{$uuid()}}")
	s = strings.ReplaceAll(s, "{{$timestamp}}", "{{$timestamp()}}")
	s = strings.ReplaceAll(s, "{{$randomInt}}", "{{$random(0, 1000)}}")
	s = strings.ReplaceAll(s, "{{$randomEmail}}", "{{$randomEmail()}}")
	s = strings.ReplaceAll(s, "{{$randomUUID}}", "{{$uuid()}}")
	return s
}

func sanitizePostmanName(name string) string {
	result := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			return r
		}
		return '_'
	}, name)
	for strings.Contains(result, "__") {
		result = strings.ReplaceAll(result, "__", "_")
	}
	result = strings.Trim(result, "_")
	if result == "" {
		return "request"
	}
	return result
}
