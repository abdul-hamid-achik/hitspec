import { get, post } from '@/api/client'
import type { MockRoute } from '@/types/api'

export interface MockStatus {
  running: boolean
  port?: number
  routes: MockRoute[]
}

export function getMockStatus(): Promise<MockStatus> {
  return get<MockStatus>('/api/v1/mock/routes')
}

export function startMock(config: { files: string[]; port?: number }): Promise<void> {
  return post('/api/v1/mock/start', config)
}

export function stopMock(): Promise<void> {
  return post('/api/v1/mock/stop')
}
