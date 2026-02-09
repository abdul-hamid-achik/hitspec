import { post } from '@/api/client'

export interface ImportResult {
  content: string
  fileName: string
}

export function importCurl(command: string): Promise<ImportResult> {
  return post<ImportResult>('/api/v1/import/curl', { command })
}

export function importInsomnia(data: string): Promise<ImportResult> {
  return post<ImportResult>('/api/v1/import/insomnia', { data })
}

export function importOpenAPI(spec: string): Promise<ImportResult> {
  return post<ImportResult>('/api/v1/import/openapi', { spec })
}

export function exportCurl(file: string, requestIndex?: number): Promise<{ commands: string[] }> {
  return post<{ commands: string[] }>('/api/v1/export/curl', { file, requestIndex })
}
