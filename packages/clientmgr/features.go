package clientmgr

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/abdul-hamid-achik/hitspec/v2/packages/contract"
	"github.com/abdul-hamid-achik/hitspec/v2/packages/core/config"
	"github.com/abdul-hamid-achik/hitspec/v2/packages/core/parser"
	"github.com/abdul-hamid-achik/hitspec/v2/packages/export/snippet"
	"github.com/abdul-hamid-achik/hitspec/v2/packages/http"
	curlimport "github.com/abdul-hamid-achik/hitspec/v2/packages/import/curl"
	"github.com/abdul-hamid-achik/hitspec/v2/packages/import/insomnia"
	"github.com/abdul-hamid-achik/hitspec/v2/packages/import/openapi"
	"github.com/abdul-hamid-achik/hitspec/v2/packages/import/postman"
	"github.com/abdul-hamid-achik/hitspec/v2/packages/mock"
	"github.com/abdul-hamid-achik/hitspec/v2/packages/proxy"
	"github.com/abdul-hamid-achik/hitspec/v2/packages/stress"
)

// StartStress starts a background stress test.
func (m *Manager) StartStress(ctx context.Context, req StressStartReq) error {
	runCfg, clientOpts := m.runSettings(req.Environment)
	envName, configEnvs := runCfg.Environment, runCfg.ConfigEnvironments
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
	// The runner resolves {{variables}} from the config environments, and its
	// default reporter prints a header, a live progress bar and a summary to
	// os.Stdout, which would be painted over the TUI. Progress reaches
	// subscribers through the stress_update events published below, so the
	// reporter writes nowhere.
	runnerOpts := []stress.RunnerOption{
		stress.WithHTTPClient(http.NewClient(clientOpts...)),
		stress.WithReporter(stress.NewReporter(stress.WithWriter(io.Discard), stress.WithNoProgress(true))),
	}
	if envName != "" {
		runnerOpts = append(runnerOpts, stress.WithEnvironment(envName))
	}
	if len(configEnvs) > 0 {
		runnerOpts = append(runnerOpts, stress.WithConfigEnvironments(configEnvs))
	}
	stressRunner := stress.NewRunner(cfg, runnerOpts...)
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
	envName, configEnvs := m.envSnapshot(req.Environment)
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
		// Request lines flow through mock_request events; the engine logs with
		// the std logger, which must not reach the TUI.
		mock.WithVerbose(false),
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
	// The mock server only knows a file's own variables, so inject the
	// environment's (a route like {{baseUrl}}/users must become /users).
	for _, path := range absFiles {
		file, err := parseWithEnv(path, envName, configEnvs)
		if err == nil {
			err = mockSrv.LoadParsedFile(file)
		} else {
			err = fmt.Errorf("failed to parse file %s: %w", path, err)
		}
		if err != nil {
			m.mu.Unlock()
			return MockStatusDTO{}, err
		}
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
		proxy.WithVerbose(false), // the engine logs via the std logger, which must not reach the TUI
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
	if req.ProviderURL == "" {
		return nil, fmt.Errorf("providerUrl is required")
	}
	runCfg, clientOpts := m.runSettings(req.Environment)
	envName, configEnvs := runCfg.Environment, runCfg.ConfigEnvironments
	opts := []contract.Option{
		contract.WithHTTPClient(http.NewClient(clientOpts...)),
		contract.WithProviderURL(req.ProviderURL),
		contract.WithVerbose(false), // verbose state-handler lines go to os.Stdout, which the TUI owns
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
		// Inject the environment's variables (the verifier only knows the
		// file's own, plus baseUrl from the provider URL).
		parsed, err := parseWithEnv(absPath, envName, configEnvs)
		var result *contract.VerificationResult
		if err != nil {
			err = fmt.Errorf("failed to parse contract file: %w", err)
		} else {
			result, err = verifier.VerifyContext(ctx, parsed)
		}
		if err != nil && ctx.Err() != nil {
			// A cancelled run is not a parse failure: stop and say so, without
			// a bogus "parse" entry for the file that was running.
			return results, ctx.Err()
		}
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
	res, err := postman.NewConverter().Convert(data)
	if err != nil {
		return ImportResultDTO{}, err
	}
	content := res.ContentWithEnvironments()
	return ImportResultDTO{Content: content, RequestCount: countRequests(content)}, nil
}

// Export renders requests as curl or another client snippet (see snippet.Formats).
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
	commands := make([]string, 0, len(reqs))
	rendered := 0
	var lastErr error
	for _, r := range reqs {
		cmd, err := snippet.Render(req.Format, r)
		if err != nil {
			// One request a format cannot express (multipart, file body) must not
			// fail a whole-file export; say so in its place, as `hitspec export` does.
			if req.RequestName != "" {
				return ExportResultDTO{}, err
			}
			lastErr = err
			cmd = snippet.Comment(req.Format) + " SKIPPED: " + err.Error()
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
