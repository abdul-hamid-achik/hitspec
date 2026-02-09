import { post } from '@/api/client'
import type { ExecuteRequest, ExecuteResult } from '@/types/api'

export function executeRequest(req: ExecuteRequest): Promise<ExecuteResult> {
  return post<ExecuteResult>('/api/v1/execute', req)
}

export function executeFile(filePath: string, environment?: string): Promise<ExecuteResult> {
  return post<ExecuteResult>('/api/v1/execute/file', { filePath, environment })
}
