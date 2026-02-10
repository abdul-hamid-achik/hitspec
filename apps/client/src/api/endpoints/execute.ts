import { post } from '@/api/client'
import type { ExecuteRequest, ExecuteResult } from '@/types/api'

export function executeRequest(req: ExecuteRequest): Promise<ExecuteResult> {
  // Use /execute when requestName is set (supports name filtering),
  // otherwise /run which runs all requests in the file.
  const endpoint = req.requestName ? '/api/v1/execute' : '/api/v1/run'
  return post<ExecuteResult>(endpoint, req)
}

export function executeFile(file: string, environment?: string): Promise<ExecuteResult> {
  return post<ExecuteResult>('/api/v1/run', { file, environment })
}
