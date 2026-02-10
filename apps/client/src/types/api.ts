export interface FileInfo {
  path: string
  name: string
  dir: string
  isDir: boolean
  children?: FileInfo[]
  requestCount?: number
}

export interface WorkspaceInfo {
  root: string
  files: FileInfo[]
  totalRequests: number
  environment: string
  hasConfig: boolean
}

export interface HeaderDTO {
  key: string
  value: string
  enabled: boolean
}

export interface BodyDTO {
  type: string
  content: string
  filePath?: string
}

export interface AssertionDTO {
  field: string
  operator: string
  expected: string
  line: number
}

export interface CaptureDTO {
  name: string
  source: string
  expression: string
  line: number
}

export interface RequestDTO {
  method: string
  url: string
  headers: HeaderDTO[]
  body?: BodyDTO
  assertions: AssertionDTO[]
  captures: CaptureDTO[]
  name: string
  tags: string[]
  line: number
  auth?: Record<string, string>
}

export interface ParsedFile {
  path: string
  requests: RequestDTO[]
  variables: Record<string, string>
  errors: string[]
}

export interface ExecuteRequest {
  file: string
  requestName?: string
  environment?: string
}

export interface AssertionResult {
  subject: string
  operator: string
  expected: unknown
  actual: unknown
  passed: boolean
  message?: string
}

export interface RunResult {
  name: string
  passed: boolean
  skipped?: boolean
  skipReason?: string
  duration: number
  error?: string
  request?: { method: string; url: string; headers?: Record<string, string> }
  response?: {
    statusCode: number
    status: string
    headers?: Record<string, string>
    body?: string
    duration: number
    size: number
  }
  assertions?: AssertionResult[]
  captures?: Record<string, unknown>
}

export interface ExecuteResult {
  file: string
  duration: number
  passed: number
  failed: number
  skipped: number
  results: RunResult[]
}

export interface EnvironmentDTO {
  name: string
  variables: Record<string, string>
  isActive: boolean
}

export interface ConfigDTO {
  timeout: number
  followRedirects: boolean
  maxRedirects: number
  insecure: boolean
  proxy?: string
  verbose: boolean
}

export interface HistoryEntry {
  id: string
  timestamp: string
  filePath: string
  requestName: string
  method: string
  url: string
  statusCode: number
  duration: number
  passed: boolean
}

export interface StressConfig {
  filePath: string
  requestIndex: number
  concurrency: number
  duration: string
  rps: number
  environment?: string
}

export interface StressMetrics {
  totalRequests: number
  successCount: number
  failCount: number
  avgLatency: number
  p50Latency: number
  p95Latency: number
  p99Latency: number
  rps: number
  elapsed: number
  statusCodes: Record<number, number>
}

export interface StressStatus {
  running: boolean
  config?: StressConfig
  metrics?: StressMetrics
}

export interface MockConfig {
  port: number
  routes: MockRoute[]
}

export interface MockRoute {
  method: string
  path: string
  status: number
  headers: Record<string, string>
  body: string
  delay: number
}

export interface ImportRequest {
  format: string
  content: string
  filePath?: string
}

export interface ExportRequest {
  filePath: string
  format: string
}

export interface SystemInfo {
  version: string
  goVersion: string
  platform: string
  workDir: string
  uptime: number
}

// --- Contract Testing ---

export interface ContractVerifyRequest {
  files: string[]
  providerUrl: string
  stateHandler?: string
}

export interface ContractInteraction {
  name: string
  provider?: string
  state?: string
  passed: boolean
  error?: string
  duration: number
}

export interface ContractResult {
  file: string
  passed: number
  failed: number
  skipped: number
  duration: number
  results: ContractInteraction[]
}

export interface ContractStatus {
  files: string[]
  results?: ContractResult[]
}

// --- Record Proxy ---

export interface RecordStartRequest {
  targetUrl: string
  port?: number
  exclude?: string[]
  sanitize?: string[]
  deduplicate?: boolean
}

export interface RecordingEntry {
  method: string
  path: string
  url: string
  contentType?: string
  statusCode?: number
  duration?: number
}

export interface RecordStatus {
  running: boolean
  targetUrl?: string
  port?: number
  count: number
  recordings?: RecordingEntry[]
}

// --- Stress Profiles ---

export interface StressProfile {
  name: string
  duration?: string
  rate?: number
  vus?: number
  maxVUs?: number
  thinkTime?: string
  rampUp?: string
  thresholds?: Record<string, string>
}

// --- Assertion Operators ---

export const ASSERTION_OPERATORS = [
  { value: '==', label: 'Equals', description: 'Exact equality' },
  { value: '!=', label: 'Not Equals', description: 'Not equal to' },
  { value: '>', label: 'Greater Than', description: 'Numeric greater than' },
  { value: '>=', label: 'Greater or Equal', description: 'Numeric greater or equal' },
  { value: '<', label: 'Less Than', description: 'Numeric less than' },
  { value: '<=', label: 'Less or Equal', description: 'Numeric less or equal' },
  { value: 'contains', label: 'Contains', description: 'String contains substring' },
  { value: 'not contains', label: 'Not Contains', description: 'String does not contain' },
  { value: 'startsWith', label: 'Starts With', description: 'String starts with' },
  { value: 'endsWith', label: 'Ends With', description: 'String ends with' },
  { value: 'matches', label: 'Matches', description: 'Regex pattern match' },
  { value: 'exists', label: 'Exists', description: 'Field exists' },
  { value: 'not exists', label: 'Not Exists', description: 'Field does not exist' },
  { value: 'length', label: 'Length', description: 'Exact length' },
  { value: 'length >', label: 'Length >', description: 'Length greater than' },
  { value: 'length >=', label: 'Length >=', description: 'Length greater or equal' },
  { value: 'length <', label: 'Length <', description: 'Length less than' },
  { value: 'length <=', label: 'Length <=', description: 'Length less or equal' },
  { value: 'includes', label: 'Includes', description: 'Array includes value' },
  { value: 'not includes', label: 'Not Includes', description: 'Array does not include' },
  { value: 'in', label: 'In', description: 'Value in list' },
  { value: 'not in', label: 'Not In', description: 'Value not in list' },
  { value: 'type', label: 'Type', description: 'JSON type check' },
  { value: 'each', label: 'Each', description: 'Every element matches' },
  { value: 'schema', label: 'Schema', description: 'JSON Schema validation' },
  { value: 'snapshot', label: 'Snapshot', description: 'Snapshot comparison' },
] as const

export const ASSERTION_SUBJECTS = [
  { value: 'status', label: 'Status Code', group: 'Response' },
  { value: 'header', label: 'Header', group: 'Response' },
  { value: 'body', label: 'Body (raw)', group: 'Body' },
  { value: 'jsonpath', label: 'JSONPath', group: 'Body' },
  { value: 'duration', label: 'Duration (ms)', group: 'Timing' },
  { value: 'size', label: 'Body Size', group: 'Timing' },
] as const

export type WSMessageType =
  | 'file_changed'
  | 'execution_start'
  | 'execution_complete'
  | 'stress_update'
  | 'mock_request'
  | 'error'
  | 'pong'

export interface WSMessage<T = unknown> {
  type: WSMessageType
  payload: T
  timestamp: string
}
