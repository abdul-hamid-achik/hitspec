package serve

import "time"

// --- Workspace & Files ---

// WorkspaceDTO describes the current workspace.
type WorkspaceDTO struct {
	Root          string            `json:"root"`
	Files         []FileTreeNodeDTO `json:"files"`
	TotalRequests int               `json:"totalRequests"`
	Environment   string            `json:"environment"`
	HasConfig     bool              `json:"hasConfig"`
}

// FileTreeNodeDTO is a node in the file tree (file or directory).
type FileTreeNodeDTO struct {
	Path         string            `json:"path"`
	Name         string            `json:"name"`
	Dir          string            `json:"dir"`
	IsDir        bool              `json:"isDir"`
	Children     []FileTreeNodeDTO `json:"children,omitempty"`
	RequestCount int               `json:"requestCount,omitempty"`
}

// FileInfoDTO describes a hitspec file (flat list).
type FileInfoDTO struct {
	Path         string `json:"path"`
	RelativePath string `json:"relativePath"`
	Name         string `json:"name"`
	Size         int64  `json:"size"`
	ModTime      string `json:"modTime"`
	RequestCount int    `json:"requestCount"`
}

// ParsedFileDTO is a fully-parsed hitspec file.
type ParsedFileDTO struct {
	Path      string        `json:"path"`
	Variables []VariableDTO `json:"variables"`
	Requests  []RequestDTO  `json:"requests"`
}

// VariableDTO is a file-level variable.
type VariableDTO struct {
	Name  string `json:"name"`
	Value string `json:"value"`
	Line  int    `json:"line"`
}

// --- Request ---

// RequestDTO represents a parsed HTTP request.
type RequestDTO struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Tags        []string       `json:"tags,omitempty"`
	Method      string         `json:"method"`
	URL         string         `json:"url"`
	Headers     []HeaderDTO    `json:"headers,omitempty"`
	QueryParams []QueryDTO     `json:"queryParams,omitempty"`
	Body        *BodyDTO       `json:"body,omitempty"`
	Assertions  []AssertionDTO `json:"assertions,omitempty"`
	Captures    []CaptureDTO   `json:"captures,omitempty"`
	Line        int            `json:"line"`
	Metadata    *MetadataDTO   `json:"metadata,omitempty"`
}

// HeaderDTO is a request header.
type HeaderDTO struct {
	Key   string `json:"key"`
	Value string `json:"value"`
	Line  int    `json:"line"`
}

// QueryDTO is a query parameter.
type QueryDTO struct {
	Key   string `json:"key"`
	Value string `json:"value"`
	Line  int    `json:"line"`
}

// BodyDTO is the request body.
type BodyDTO struct {
	ContentType string `json:"contentType"`
	Raw         string `json:"raw,omitempty"`
	Line        int    `json:"line"`
}

// AssertionDTO is an assertion on a response.
type AssertionDTO struct {
	Subject  string `json:"subject"`
	Operator string `json:"operator"`
	Expected any    `json:"expected"`
	Line     int    `json:"line"`
}

// CaptureDTO is a captured value.
type CaptureDTO struct {
	Name   string `json:"name"`
	Source string `json:"source"`
	Path   string `json:"path,omitempty"`
	Line   int    `json:"line"`
}

// MetadataDTO is request metadata/annotations.
type MetadataDTO struct {
	Skip    string   `json:"skip,omitempty"`
	Only    bool     `json:"only,omitempty"`
	Timeout int      `json:"timeout,omitempty"`
	Retry   int      `json:"retry,omitempty"`
	Depends []string `json:"depends,omitempty"`
	Auth    *AuthDTO `json:"auth,omitempty"`
}

// AuthDTO is authentication config.
type AuthDTO struct {
	Type   string   `json:"type"`
	Params []string `json:"params,omitempty"`
}

// --- Execution ---

// ExecuteReq is the request body for POST /execute.
type ExecuteReq struct {
	File        string `json:"file"`
	RequestName string `json:"requestName,omitempty"`
	Environment string `json:"environment,omitempty"`
}

// RunReq is the request body for POST /run.
type RunReq struct {
	File        string `json:"file"`
	Environment string `json:"environment,omitempty"`
}

// RunResultDTO holds results from running a file.
type RunResultDTO struct {
	File     string            `json:"file"`
	Duration float64           `json:"duration"`
	Passed   int               `json:"passed"`
	Failed   int               `json:"failed"`
	Skipped  int               `json:"skipped"`
	Results  []RequestResultDTO `json:"results"`
}

// RequestResultDTO holds results from a single request execution.
type RequestResultDTO struct {
	Name       string              `json:"name"`
	Passed     bool                `json:"passed"`
	Skipped    bool                `json:"skipped,omitempty"`
	SkipReason string              `json:"skipReason,omitempty"`
	Duration   float64             `json:"duration"`
	Error      string              `json:"error,omitempty"`
	Request    *HTTPRequestDTO     `json:"request,omitempty"`
	Response   *HTTPResponseDTO    `json:"response,omitempty"`
	Assertions []AssertionResultDTO `json:"assertions,omitempty"`
	Captures   map[string]any      `json:"captures,omitempty"`
}

// HTTPRequestDTO is the executed HTTP request.
type HTTPRequestDTO struct {
	Method  string            `json:"method"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers,omitempty"`
}

// HTTPResponseDTO is the received HTTP response.
type HTTPResponseDTO struct {
	StatusCode int               `json:"statusCode"`
	Status     string            `json:"status"`
	Headers    map[string]string `json:"headers,omitempty"`
	Body       string            `json:"body,omitempty"`
	Duration   float64           `json:"duration"`
	Size       int64             `json:"size"`
}

// AssertionResultDTO is the result of evaluating one assertion.
type AssertionResultDTO struct {
	Subject  string `json:"subject"`
	Operator string `json:"operator"`
	Expected any    `json:"expected"`
	Actual   any    `json:"actual"`
	Passed   bool   `json:"passed"`
	Message  string `json:"message,omitempty"`
}

// --- Environment ---

// EnvironmentDTO is an environment with its variables.
type EnvironmentDTO struct {
	Name      string         `json:"name"`
	Variables map[string]any `json:"variables"`
}

// --- Config ---

// ConfigDTO is the hitspec.yaml configuration.
type ConfigDTO struct {
	DefaultEnvironment string            `json:"defaultEnvironment,omitempty"`
	Timeout            int               `json:"timeout,omitempty"`
	Retries            int               `json:"retries,omitempty"`
	FollowRedirects    *bool             `json:"followRedirects,omitempty"`
	ValidateSSL        *bool             `json:"validateSSL,omitempty"`
	Proxy              string            `json:"proxy,omitempty"`
	Headers            map[string]string `json:"headers,omitempty"`
	Parallel           *bool             `json:"parallel,omitempty"`
	Concurrency        int               `json:"concurrency,omitempty"`
}

// --- History ---

// HistoryEntryDTO is a single execution history entry.
type HistoryEntryDTO struct {
	ID          string  `json:"id"`
	File        string  `json:"file"`
	RequestName string  `json:"requestName,omitempty"`
	Method      string  `json:"method"`
	URL         string  `json:"url"`
	StatusCode  int     `json:"statusCode"`
	Duration    float64 `json:"duration"`
	Passed      bool    `json:"passed"`
	Timestamp   string  `json:"timestamp"`
}

// --- Stress ---

// StressStartReq is the request body for POST /stress/start.
type StressStartReq struct {
	Files    []string `json:"files"`
	Duration string   `json:"duration"`
	Rate     float64  `json:"rate,omitempty"`
	VUs      int      `json:"vus,omitempty"`
	MaxVUs   int      `json:"maxVUs,omitempty"`
}

// StressStatusDTO is the current stress test status.
type StressStatusDTO struct {
	Running bool             `json:"running"`
	Elapsed float64          `json:"elapsed"`
	Stats   *StressStatsDTO  `json:"stats,omitempty"`
}

// StressStatsDTO holds real-time stress metrics.
type StressStatsDTO struct {
	Total     int64   `json:"total"`
	Success   int64   `json:"success"`
	Errors    int64   `json:"errors"`
	RPS       float64 `json:"rps"`
	P50Ms     float64 `json:"p50Ms"`
	P95Ms     float64 `json:"p95Ms"`
	P99Ms     float64 `json:"p99Ms"`
	MaxMs     float64 `json:"maxMs"`
	ErrorRate float64 `json:"errorRate"`
	ActiveVUs int32   `json:"activeVUs"`
}

// --- Mock ---

// MockStartReq is the request body for POST /mock/start.
type MockStartReq struct {
	Files []string `json:"files"`
	Port  int      `json:"port,omitempty"`
	Delay string   `json:"delay,omitempty"`
}

// MockRouteDTO is a mock server route.
type MockRouteDTO struct {
	Method      string `json:"method"`
	Path        string `json:"path"`
	Name        string `json:"name,omitempty"`
	StatusCode  int    `json:"statusCode"`
	ContentType string `json:"contentType"`
}

// MockStatusDTO is the current mock server status.
type MockStatusDTO struct {
	Running bool           `json:"running"`
	Port    int            `json:"port,omitempty"`
	Routes  []MockRouteDTO `json:"routes,omitempty"`
}

// --- Import/Export ---

// ImportCurlReq is the request body for POST /import/curl.
type ImportCurlReq struct {
	Command  string `json:"command,omitempty"`
	FilePath string `json:"filePath,omitempty"`
}

// ImportInsomniaReq is the request body for POST /import/insomnia.
type ImportInsomniaReq struct {
	Data     string `json:"data,omitempty"`
	FilePath string `json:"filePath,omitempty"`
}

// ImportOpenAPIReq is the request body for POST /import/openapi.
type ImportOpenAPIReq struct {
	SpecPath string `json:"specPath"`
	BaseURL  string `json:"baseUrl,omitempty"`
}

// ImportResultDTO is the result of an import operation.
type ImportResultDTO struct {
	Content      string `json:"content"`
	RequestCount int    `json:"requestCount"`
}

// ExportCurlReq is the request body for POST /export/curl.
type ExportCurlReq struct {
	File        string `json:"file"`
	RequestName string `json:"requestName,omitempty"`
}

// ExportResultDTO is the result of an export operation.
type ExportResultDTO struct {
	Commands []string `json:"commands"`
}

// --- System ---

// SystemInfoDTO contains version and build information.
type SystemInfoDTO struct {
	Version   string `json:"version"`
	BuildTime string `json:"buildTime"`
	GoVersion string `json:"goVersion"`
	OS        string `json:"os"`
	Arch      string `json:"arch"`
}

// --- WebSocket ---

// WSMessage is the WebSocket message envelope.
type WSMessage struct {
	Type    string `json:"type"`
	Payload any    `json:"payload"`
}

// WSFileEvent is a file change event.
type WSFileEvent struct {
	Path      string `json:"path"`
	Operation string `json:"operation"`
	Timestamp string `json:"timestamp"`
}

// WSExecEvent is an execution lifecycle event.
type WSExecEvent struct {
	ID        string          `json:"id"`
	File      string          `json:"file"`
	Status    string          `json:"status"`
	Result    *RunResultDTO   `json:"result,omitempty"`
	Error     string          `json:"error,omitempty"`
	Timestamp string          `json:"timestamp"`
}

// WSStressMetrics is a stress metrics broadcast.
type WSStressMetrics struct {
	Stats     StressStatsDTO `json:"stats"`
	Elapsed   float64        `json:"elapsed"`
	Timestamp string         `json:"timestamp"`
}

// WSMockEvent is a mock server event.
type WSMockEvent struct {
	Event     string        `json:"event"`
	Method    string        `json:"method,omitempty"`
	Path      string        `json:"path,omitempty"`
	Status    int           `json:"status,omitempty"`
	Duration  float64       `json:"duration,omitempty"`
	Timestamp string        `json:"timestamp"`
}

// Typed timestamp helper.
func nowISO() string {
	return time.Now().UTC().Format(time.RFC3339)
}
