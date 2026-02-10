import { get, post, del } from '@/api/client'
import type { RecordStartRequest, RecordStatus } from '@/types/api'

export function getRecordStatus(): Promise<RecordStatus> {
  return get<RecordStatus>('/api/v1/record/status')
}

export function startRecording(req: RecordStartRequest): Promise<void> {
  return post('/api/v1/record/start', req)
}

export function stopRecording(): Promise<void> {
  return post('/api/v1/record/stop')
}

export function exportRecordings(): Promise<{ content: string }> {
  return post<{ content: string }>('/api/v1/record/export')
}

export function clearRecordings(): Promise<void> {
  return del('/api/v1/record/clear')
}
