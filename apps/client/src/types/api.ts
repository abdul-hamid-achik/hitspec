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
  line: number
}

export interface QueryDTO {
  key: string
  value: string
  line: number
}

export interface BodyDTO {
  contentType: string
  raw?: string
  line: number
}

export interface AssertionDTO {
  subject: string
  operator: string
  expected: unknown
  line: number
}

export interface CaptureDTO {
  name: string
  source: string
  path?: string
  line: number
}

export interface AuthDTO {
  type: string
  params?: string[]
}

export interface MetadataDTO {
  skip?: string
  only?: boolean
  timeout?: number
  retry?: number
  depends?: string[]
  auth?: AuthDTO
}

export interface RequestDTO {
  name: string
  description?: string
  tags?: string[]
  method: string
  url: string
  headers?: HeaderDTO[]
  queryParams?: QueryDTO[]
  body?: BodyDTO
  assertions?: AssertionDTO[]
  captures?: CaptureDTO[]
  line: number
  metadata?: MetadataDTO
}

export interface VariableDTO {
  name: string
  value: string
  line: number
}

export interface ParsedFile {
  path: string
  variables: VariableDTO[]
  requests: RequestDTO[]
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

export interface SSEEventDTO {
  id?: string
  type?: string
  data: string
}

export interface RunResult {
  name: string
  description?: string
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
  sseEvents?: SSEEventDTO[]
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
  variables: Record<string, unknown>
}

export interface ConfigDTO {
  defaultEnvironment?: string
  timeout?: number
  retries?: number
  followRedirects?: boolean
  validateSSL?: boolean
  proxy?: string
  headers?: Record<string, string>
  parallel?: boolean
  concurrency?: number
}

export interface HistoryEntry {
  id: string
  timestamp: string
  file: string
  requestName?: string
  method: string
  url: string
  statusCode: number
  duration: number
  passed: boolean
}

export interface StressStartRequest {
  files: string[]
  duration: string
  rate?: number
  vus?: number
  maxVUs?: number
}

export interface StressStatsDTO {
  total: number
  success: number
  errors: number
  rps: number
  p50Ms: number
  p95Ms: number
  p99Ms: number
  maxMs: number
  errorRate: number
  activeVUs: number
}

export interface StressStatus {
  running: boolean
  elapsed: number
  stats?: StressStatsDTO
}

export interface MockRouteDTO {
  method: string
  path: string
  name?: string
  statusCode: number
  contentType: string
}

export interface MockStatusDTO {
  running: boolean
  port?: number
  routes?: MockRouteDTO[]
}

export interface ImportResultDTO {
  content: string
  requestCount: number
}

export interface ImportCurlRequest {
  command?: string
  filePath?: string
}

export interface ImportInsomniaRequest {
  data?: string
  filePath?: string
}

export interface ImportOpenAPIRequest {
  specPath: string
  baseUrl?: string
}

export interface ExportCurlRequest {
  file: string
  requestName?: string
}

export interface ExportResultDTO {
  commands: string[]
}

export interface SystemInfo {
  version: string
  buildTime: string
  goVersion: string
  os: string
  arch: string
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

export interface ContractStatusDTO {
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
  | 'environment_changed'
  | 'error'
  | 'pong'

export interface WSMessage<T = unknown> {
  type: WSMessageType
  payload: T
  timestamp: string
}
