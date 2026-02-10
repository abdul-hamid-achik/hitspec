import { post } from '@/api/client'
import type { ImportResultDTO, ExportResultDTO } from '@/types/api'

export function importCurl(command: string): Promise<ImportResultDTO> {
  return post<ImportResultDTO>('/api/v1/import/curl', { command })
}

export function importInsomnia(data: string): Promise<ImportResultDTO> {
  return post<ImportResultDTO>('/api/v1/import/insomnia', { data })
}

export function importOpenAPI(specPath: string, baseUrl?: string): Promise<ImportResultDTO> {
  return post<ImportResultDTO>('/api/v1/import/openapi', { specPath, baseUrl })
}

export function exportCurl(file: string, requestName?: string): Promise<ExportResultDTO> {
  return post<ExportResultDTO>('/api/v1/export/curl', { file, requestName })
}
