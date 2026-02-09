import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import * as client from '@/api/client'
import { getWorkspace as getFilesWorkspace, getFile } from '@/api/endpoints/files'
import { getEnvironments } from '@/api/endpoints/environments'
import { getConfig } from '@/api/endpoints/config'
import { getHistory } from '@/api/endpoints/history'
import { getStressStatus, startStress, stopStress } from '@/api/endpoints/stress'
import { importCurl } from '@/api/endpoints/import'
import { getSystemInfo } from '@/api/endpoints/system'
import { getWorkspace } from '@/api/endpoints/workspace'

describe('API Endpoints', () => {
  beforeEach(() => {
    vi.spyOn(client, 'get').mockResolvedValue({} as never)
    vi.spyOn(client, 'post').mockResolvedValue({} as never)
    vi.spyOn(client, 'put').mockResolvedValue({} as never)
    vi.spyOn(client, 'del').mockResolvedValue(undefined as never)
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('getWorkspace (workspace endpoint) calls correct endpoint', async () => {
    await getWorkspace()
    expect(client.get).toHaveBeenCalledWith('/api/v1/workspace')
  })

  it('getWorkspace (files endpoint) calls workspace endpoint', async () => {
    await getFilesWorkspace()
    expect(client.get).toHaveBeenCalledWith('/api/v1/workspace')
  })

  it('getFile encodes path', async () => {
    await getFile('dir/test.http')
    expect(client.get).toHaveBeenCalledWith('/api/v1/files/dir%2Ftest.http')
  })

  it('getEnvironments calls correct endpoint', async () => {
    await getEnvironments()
    expect(client.get).toHaveBeenCalledWith('/api/v1/environments')
  })

  it('getConfig calls correct endpoint', async () => {
    await getConfig()
    expect(client.get).toHaveBeenCalledWith('/api/v1/config')
  })

  it('getHistory calls correct endpoint', async () => {
    await getHistory()
    expect(client.get).toHaveBeenCalledWith('/api/v1/history')
  })

  it('getStressStatus calls correct endpoint', async () => {
    await getStressStatus()
    expect(client.get).toHaveBeenCalledWith('/api/v1/stress/status')
  })

  it('startStress sends config', async () => {
    const config = { filePath: 'test.http', requestIndex: 0, concurrency: 10, duration: '30s', rps: 50 }
    await startStress(config)
    expect(client.post).toHaveBeenCalledWith('/api/v1/stress/start', config)
  })

  it('stopStress calls correct endpoint', async () => {
    await stopStress()
    expect(client.post).toHaveBeenCalledWith('/api/v1/stress/stop')
  })

  it('importCurl sends command', async () => {
    await importCurl('curl http://example.com')
    expect(client.post).toHaveBeenCalledWith('/api/v1/import/curl', { command: 'curl http://example.com' })
  })

  it('getSystemInfo calls correct endpoint', async () => {
    await getSystemInfo()
    expect(client.get).toHaveBeenCalledWith('/api/v1/system/info')
  })
})
