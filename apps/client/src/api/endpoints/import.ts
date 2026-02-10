import { post } from '@/api/client'
import type { ImportResultDTO } from '@/types/api'

export function importCurl(command: string): Promise<ImportResultDTO> {
  return post<ImportResultDTO>('/api/v1/import/curl', { command })
}

export function importInsomnia(data: string): Promise<ImportResultDTO> {
  return post<ImportResultDTO>('/api/v1/import/insomnia', { data })
}

export function importOpenAPI(specPath: string, baseUrl?: string): Promise<ImportResultDTO> {
  return post<ImportResultDTO>('/api/v1/import/openapi', { specPath, baseUrl })
}
