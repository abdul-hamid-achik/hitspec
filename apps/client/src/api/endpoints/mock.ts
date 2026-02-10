import { get, post } from '@/api/client'
import type { MockStatusDTO } from '@/types/api'

export function getMockStatus(): Promise<MockStatusDTO> {
  return get<MockStatusDTO>('/api/v1/mock/routes')
}

export function startMock(config: { files: string[]; port?: number; delay?: string }): Promise<MockStatusDTO> {
  return post<MockStatusDTO>('/api/v1/mock/start', config)
}

export function stopMock(): Promise<void> {
  return post('/api/v1/mock/stop')
}
