import { post } from '@/api/client'
import type { ExportRequest } from '@/types/api'

export function exportFile(req: ExportRequest): Promise<Blob> {
  const url = '/api/v1/export'
  return fetch(url, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(req),
  }).then((res) => {
    if (!res.ok) throw new Error(`Export failed: ${res.statusText}`)
    return res.blob()
  })
}

export function exportToCurl(filePath: string, requestIndex: number): Promise<string> {
  return post<string>('/api/v1/export/curl', { filePath, requestIndex })
}
