import { post } from '@/api/client'
import type { ExecuteRequest, ExecuteResult } from '@/types/api'

export function executeRequest(req: ExecuteRequest): Promise<ExecuteResult> {
  return post<ExecuteResult>('/api/v1/run', req)
}

export function executeFile(file: string, environment?: string): Promise<ExecuteResult> {
  return post<ExecuteResult>('/api/v1/run', { file, environment })
}
