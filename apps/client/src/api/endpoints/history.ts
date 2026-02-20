import { get, del } from '@/api/client'
import type { HistoryEntry, HistoryList, HistoryRun, HistoryResult, HistoryResultsByRequest } from '@/types/api'

// Legacy in-memory history
export function getHistory(): Promise<HistoryEntry[]> {
  return get<HistoryEntry[]>('/api/v1/history')
}

export function clearHistory(): Promise<void> {
  return del('/api/v1/history')
}

// Persistent history (SQLite-backed)

export function fetchRuns(limit = 20, offset = 0): Promise<HistoryList> {
  return get<HistoryList>(`/api/v1/history/runs?limit=${limit}&offset=${offset}`)
}

export function fetchRunDetails(id: number): Promise<{ run: HistoryRun; results: HistoryResult[] }> {
  return get<{ run: HistoryRun; results: HistoryResult[] }>(`/api/v1/history/runs/${id}`)
}

export function clearAllHistory(): Promise<void> {
  return del('/api/v1/history/runs')
}

export function deleteRun(id: number): Promise<void> {
  return del(`/api/v1/history/runs/${id}`)
}

export function fetchResultsByRequest(
  requestName: string,
  filePath: string,
  limit = 20,
  offset = 0,
): Promise<HistoryResultsByRequest> {
  const params = new URLSearchParams({
    requestName,
    filePath,
    limit: String(limit),
    offset: String(offset),
  })
  return get<HistoryResultsByRequest>(`/api/v1/history/results?${params}`)
}
