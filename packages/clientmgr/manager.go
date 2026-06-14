package clientmgr

import (
	"context"
	"crypto/rand"
	"fmt"
	"log/slog"
	"math/big"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"

	"github.com/abdul-hamid-achik/hitspec/packages/core/config"
	"github.com/abdul-hamid-achik/hitspec/packages/core/parser"
	"github.com/abdul-hamid-achik/hitspec/packages/core/runner"
	"github.com/abdul-hamid-achik/hitspec/packages/history"
	"github.com/abdul-hamid-achik/hitspec/packages/mock"
	"github.com/abdul-hamid-achik/hitspec/packages/proxy"
	"github.com/abdul-hamid-achik/hitspec/packages/stress"
)

const (
	maxHistoryEntries = 100
	maxRequestBody    = 10 * 1024 * 1024
)

// Manager owns the API Client Manager state independent of any transport.
type Manager struct {
	config       *Config
	history      *memoryHistory
	historyStore *history.Store
	fileConfig   *config.Config
	configPath   string
	logger       *slog.Logger

	configMu sync.RWMutex
	mu       sync.Mutex

	stressRunner     *stress.Runner
	stressCancel     context.CancelFunc
	lastStressResult *StressResultDTO
	mockServer       *mock.Server
	mockCancel       context.CancelFunc
	mockPort         int
	recorder         *proxy.Recorder
	recorderCancel   context.CancelFunc
	recorderPort     int
	recorderTarget   string

	ctx    context.Context
	cancel context.CancelFunc

	watchSuppress *watchSuppressor

	subMu       sync.RWMutex
	subscribers map[chan Event]struct{}

	Version   string
	BuildTime string
}

// New creates a Manager.
func New(opts ...Option) *Manager {
	cfg := DefaultConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	absWorkDir, err := filepath.Abs(cfg.WorkDir)
	if err == nil {
		cfg.WorkDir = absWorkDir
	}

	var configPath string
	var fileConfig *config.Config
	if cfg.ConfigPath != "" {
		fileConfig, _ = config.LoadConfig(cfg.ConfigPath)
		configPath = cfg.ConfigPath
	}
	if fileConfig == nil {
		configPath = config.FindConfigPath(cfg.WorkDir)
		fileConfig, _ = config.FindAndLoadConfig(cfg.WorkDir)
	}
	if fileConfig != nil && fileConfig.DefaultEnvironment != "" && cfg.Env == "dev" {
		cfg.Env = fileConfig.DefaultEnvironment
	}

	m := &Manager{
		config:      cfg,
		history:     newMemoryHistory(),
		fileConfig:  fileConfig,
		configPath:  configPath,
		logger:      newLogger(cfg),
		subscribers: make(map[chan Event]struct{}),
	}
	m.openHistoryStore()
	return m
}

// Config returns the active manager configuration.
func (m *Manager) Config() Config {
	m.configMu.RLock()
	defer m.configMu.RUnlock()
	return *m.config
}

// Start initializes background services. It returns immediately.
func (m *Manager) Start(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}
	m.ctx, m.cancel = context.WithCancel(ctx)
	if m.config.Watch {
		m.startWatcher()
	}
}

// Close stops all background services and closes persistent storage.
func (m *Manager) Close() error {
	if m.cancel != nil {
		m.cancel()
	}
	m.mu.Lock()
	if m.stressCancel != nil {
		m.stressCancel()
	}
	if m.mockCancel != nil {
		m.mockCancel()
	}
	if m.recorderCancel != nil {
		m.recorderCancel()
	}
	m.mu.Unlock()

	m.subMu.Lock()
	for ch := range m.subscribers {
		close(ch)
		delete(m.subscribers, ch)
	}
	m.subMu.Unlock()

	if m.historyStore != nil {
		return m.historyStore.Close()
	}
	return nil
}

// Subscribe returns a channel that receives manager events until ctx is done.
func (m *Manager) Subscribe(ctx context.Context) <-chan Event {
	ch := make(chan Event, 256)
	m.subMu.Lock()
	m.subscribers[ch] = struct{}{}
	m.subMu.Unlock()
	go func() {
		<-ctx.Done()
		m.subMu.Lock()
		if _, ok := m.subscribers[ch]; ok {
			delete(m.subscribers, ch)
			close(ch)
		}
		m.subMu.Unlock()
	}()
	return ch
}

// SystemInfo returns version and platform metadata.
func (m *Manager) SystemInfo() SystemInfoDTO {
	return SystemInfoDTO{
		Version:   m.Version,
		BuildTime: m.BuildTime,
		GoVersion: runtime.Version(),
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
	}
}

func (m *Manager) publish(msgType string, payload any) {
	ev := Event{Type: msgType, Payload: payload, Timestamp: nowISO()}
	m.subMu.RLock()
	defer m.subMu.RUnlock()
	for ch := range m.subscribers {
		select {
		case ch <- ev:
		default:
		}
	}
}

func (m *Manager) requireWritable() error {
	if m.config.ReadOnly {
		return fmt.Errorf("manager is in read-only mode")
	}
	return nil
}

func (m *Manager) openHistoryStore() {
	dbPath := m.config.HistoryDBPath
	if dbPath == "" {
		home, err := os.UserHomeDir()
		if err == nil {
			dir := filepath.Join(home, ".hitspec")
			_ = os.MkdirAll(dir, 0o755)
			dbPath = filepath.Join(dir, "history.db")
		}
	}
	if dbPath == "" {
		return
	}
	_ = os.MkdirAll(filepath.Dir(dbPath), 0o755)
	store, err := history.NewStore(dbPath)
	if err != nil {
		m.logger.Warn("failed to open history database", "error", err, "path", dbPath)
		return
	}
	m.historyStore = store
}

func (m *Manager) absPath(relPath string) (string, error) {
	if relPath == "" {
		return "", fmt.Errorf("path is required")
	}
	absPath := relPath
	if !filepath.IsAbs(absPath) {
		absPath = filepath.Join(m.config.WorkDir, relPath)
	}
	if !isPathWithin(m.config.WorkDir, absPath) {
		return "", fmt.Errorf("path outside workspace: %s", relPath)
	}
	return absPath, nil
}

func (m *Manager) relPath(path string) string {
	rel, err := filepath.Rel(m.config.WorkDir, path)
	if err != nil || rel == "" {
		return path
	}
	return rel
}

func (m *Manager) getConfigEnvsLocked() map[string]map[string]any {
	if m.fileConfig != nil {
		return m.fileConfig.Environments
	}
	return nil
}

func (m *Manager) saveConfigLocked() error {
	if m.configPath == "" {
		m.configPath = filepath.Join(m.config.WorkDir, "hitspec.yaml")
	}
	return m.fileConfig.SaveConfig(m.configPath)
}

type memoryHistory struct {
	mu      sync.RWMutex
	entries []HistoryEntryDTO
}

func newMemoryHistory() *memoryHistory {
	return &memoryHistory{entries: make([]HistoryEntryDTO, 0, maxHistoryEntries)}
}

func (h *memoryHistory) add(entry HistoryEntryDTO) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.entries) >= maxHistoryEntries {
		h.entries = h.entries[1:]
	}
	h.entries = append(h.entries, entry)
}

func (h *memoryHistory) entriesCopy() []HistoryEntryDTO {
	h.mu.RLock()
	defer h.mu.RUnlock()
	out := make([]HistoryEntryDTO, len(h.entries))
	copy(out, h.entries)
	return out
}

func (h *memoryHistory) clear() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.entries = h.entries[:0]
}

const idChars = "abcdefghijklmnopqrstuvwxyz0123456789"

func generateID() string {
	b := make([]byte, 8)
	max := big.NewInt(int64(len(idChars)))
	for i := range b {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			b[i] = idChars[0]
			continue
		}
		b[i] = idChars[n.Int64()]
	}
	return string(b)
}

func isPathWithin(base, target string) bool {
	absBase, err := filepath.Abs(base)
	if err != nil {
		return false
	}
	absTarget, err := filepath.Abs(target)
	if err != nil {
		return false
	}
	if resolved, err := filepath.EvalSymlinks(absBase); err == nil {
		absBase = resolved
	}
	if resolved, err := filepath.EvalSymlinks(absTarget); err == nil {
		absTarget = resolved
	} else if rel, relErr := filepath.Rel(base, target); relErr == nil {
		absTarget = filepath.Join(absBase, rel)
	}
	rel, err := filepath.Rel(absBase, absTarget)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

func collectHitspecFiles(dir string) ([]string, error) {
	var files []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && isHitspecFile(path) {
			files = append(files, path)
		}
		return nil
	})
	sort.Strings(files)
	return files, err
}

func isHitspecFile(path string) bool {
	ext := filepath.Ext(path)
	return ext == ".http" || ext == ".hitspec"
}

func countRequests(content string) int {
	return strings.Count(content, "###")
}

func newLogger(cfg *Config) *slog.Logger {
	level := slog.LevelInfo
	switch strings.ToLower(cfg.LogLevel) {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}
	opts := &slog.HandlerOptions{Level: level}
	if strings.ToLower(cfg.LogFormat) == "json" {
		return slog.New(slog.NewJSONHandler(os.Stderr, opts))
	}
	return slog.New(slog.NewTextHandler(os.Stderr, opts))
}

var sensitiveHeaders = map[string]bool{
	"authorization":       true,
	"proxy-authorization": true,
	"cookie":              true,
	"set-cookie":          true,
	"x-api-key":           true,
	"x-auth-token":        true,
}

func redactHeaders(headers map[string]string) map[string]string {
	if headers == nil {
		return nil
	}
	redacted := make(map[string]string, len(headers))
	for k, v := range headers {
		if sensitiveHeaders[strings.ToLower(k)] {
			redacted[k] = "[REDACTED]"
		} else {
			redacted[k] = v
		}
	}
	return redacted
}

func convertFile(f *parser.File) *ParsedFileDTO {
	dto := &ParsedFileDTO{
		Path:      f.Path,
		Variables: make([]VariableDTO, 0, len(f.Variables)),
		Requests:  make([]RequestDTO, 0, len(f.Requests)),
	}
	for _, v := range f.Variables {
		dto.Variables = append(dto.Variables, VariableDTO{Name: v.Name, Value: v.Value, Line: v.Line})
	}
	for _, r := range f.Requests {
		dto.Requests = append(dto.Requests, convertRequest(r))
	}
	return dto
}

func convertRequest(r *parser.Request) RequestDTO {
	dto := RequestDTO{
		Name:        r.Name,
		Description: r.Description,
		Tags:        r.Tags,
		Method:      r.Method,
		URL:         r.URL,
		Line:        r.Line,
	}
	for _, h := range r.Headers {
		dto.Headers = append(dto.Headers, HeaderDTO{Key: h.Key, Value: h.Value, Line: h.Line})
	}
	for _, q := range r.QueryParams {
		dto.QueryParams = append(dto.QueryParams, QueryDTO{Key: q.Key, Value: q.Value, Line: q.Line})
	}
	if r.Body != nil {
		dto.Body = &BodyDTO{
			ContentType: bodyTypeString(r.Body.ContentType),
			Raw:         r.Body.Raw,
			Line:        r.Body.Line,
		}
		if r.Body.GraphQL != nil {
			dto.Body.GraphQL = r.Body.GraphQL.Query
			dto.Body.Variables = r.Body.GraphQL.Variables
			if dto.Body.Raw == "" {
				dto.Body.Raw = r.Body.GraphQL.Query
			}
		}
	}
	for _, a := range r.Assertions {
		dto.Assertions = append(dto.Assertions, AssertionDTO{
			Subject:  a.Subject,
			Operator: a.Operator.String(),
			Expected: a.Expected,
			Line:     a.Line,
		})
	}
	for _, c := range r.Captures {
		dto.Captures = append(dto.Captures, CaptureDTO{
			Name:   c.Name,
			Source: c.Source.String(),
			Path:   c.Path,
			Line:   c.Line,
		})
	}
	if r.Metadata != nil {
		dto.Metadata = convertMetadata(r.Metadata)
	}
	return dto
}

func convertMetadata(md *parser.RequestMetadata) *MetadataDTO {
	dto := &MetadataDTO{
		Skip:    md.Skip,
		Only:    md.Only,
		Timeout: md.Timeout,
		Retry:   md.Retry,
		Depends: md.Depends,
	}
	if md.Auth != nil {
		dto.Auth = &AuthDTO{
			Type:   authTypeString(md.Auth.Type),
			Params: md.Auth.Params,
		}
	}
	return dto
}

func convertRunResult(r *runner.RunResult) *RunResultDTO {
	dto := &RunResultDTO{
		File:     r.File,
		Duration: float64(r.Duration.Milliseconds()),
		Passed:   r.Passed,
		Failed:   r.Failed,
		Skipped:  r.Skipped,
		Results:  make([]RequestResultDTO, 0, len(r.Results)),
	}
	for _, rr := range r.Results {
		dto.Results = append(dto.Results, convertRequestResult(rr))
	}
	return dto
}

func convertRequestResult(rr *runner.RequestResult) RequestResultDTO {
	dto := RequestResultDTO{
		Name:        rr.Name,
		Description: rr.Description,
		Passed:      rr.Passed,
		Skipped:     rr.Skipped,
		SkipReason:  rr.SkipReason,
		Duration:    float64(rr.Duration.Milliseconds()),
		Captures:    rr.Captures,
	}
	if rr.Error != nil {
		dto.Error = rr.Error.Error()
	}
	if rr.Request != nil {
		dto.Request = &HTTPRequestDTO{
			Method:  rr.Request.Method,
			URL:     rr.Request.URL,
			Headers: redactHeaders(rr.Request.Headers),
		}
	}
	if rr.Response != nil {
		dto.Response = &HTTPResponseDTO{
			StatusCode: rr.Response.StatusCode,
			Status:     rr.Response.Status,
			Headers:    redactHeaders(rr.Response.Headers),
			Body:       string(rr.Response.Body),
			Duration:   float64(rr.Response.Duration.Milliseconds()),
			Size:       int64(len(rr.Response.Body)),
		}
	}
	for _, a := range rr.Assertions {
		dto.Assertions = append(dto.Assertions, AssertionResultDTO{
			Subject:  a.Subject,
			Operator: a.Operator,
			Expected: a.Expected,
			Actual:   a.Actual,
			Passed:   a.Passed,
			Message:  a.Message,
		})
	}
	for _, ev := range rr.SSEEvents {
		dto.SSEEvents = append(dto.SSEEvents, SSEEventDTO{ID: ev.ID, Type: ev.Type, Data: ev.Data})
	}
	return dto
}

func bodyTypeString(bt parser.BodyType) string {
	switch bt {
	case parser.BodyJSON:
		return "json"
	case parser.BodyForm:
		return "form"
	case parser.BodyFormBlock:
		return "formBlock"
	case parser.BodyMultipart:
		return "multipart"
	case parser.BodyRaw:
		return "raw"
	case parser.BodyXML:
		return "xml"
	case parser.BodyGraphQL:
		return "graphql"
	default:
		return "none"
	}
}

func authTypeString(at parser.AuthType) string {
	switch at {
	case parser.AuthBasic:
		return "basic"
	case parser.AuthBearer:
		return "bearer"
	case parser.AuthAPIKey:
		return "apiKey"
	case parser.AuthAPIKeyQuery:
		return "apiKeyQuery"
	case parser.AuthDigest:
		return "digest"
	case parser.AuthAWS:
		return "aws"
	case parser.AuthOAuth2ClientCredentials:
		return "oauth2_client_credentials"
	case parser.AuthOAuth2Password:
		return "oauth2_password"
	default:
		return "none"
	}
}
