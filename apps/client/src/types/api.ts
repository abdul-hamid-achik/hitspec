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
  filePath: string
  requestIndex: number
  environment?: string
  variables?: Record<string, string>
}

export interface AssertionResult {
  field: string
  operator: string
  expected: string
  actual: string
  passed: boolean
  message: string
  line: number
}

export interface CaptureResult {
  name: string
  value: string
}

export interface RunResult {
  requestName: string
  method: string
  url: string
  statusCode: number
  duration: number
  headers: Record<string, string[]>
  body: string
  bodySize: number
  assertions: AssertionResult[]
  captures: CaptureResult[]
  error?: string
  passed: boolean
}

export interface ExecuteResult {
  results: RunResult[]
  totalDuration: number
  passed: number
  failed: number
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
