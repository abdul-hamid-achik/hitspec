import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { get, post, put, del, ApiError } from '@/api/client'

describe('API Client', () => {
  const mockFetch = vi.fn()

  beforeEach(() => {
    vi.stubGlobal('fetch', mockFetch)
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  function mockResponse(status: number, body: unknown, ok = true) {
    mockFetch.mockResolvedValueOnce({
      ok,
      status,
      statusText: status === 200 ? 'OK' : 'Error',
      json: () => Promise.resolve(body),
      text: () => Promise.resolve(JSON.stringify(body)),
    })
  }

  describe('get', () => {
    it('should make a GET request and return JSON', async () => {
      mockResponse(200, { version: '1.0.0' })

      const result = await get<{ version: string }>('/api/v1/system/info')

      expect(result).toEqual({ version: '1.0.0' })
      expect(mockFetch).toHaveBeenCalledWith(
        '/api/v1/system/info',
        expect.objectContaining({ method: 'GET' })
      )
    })

    it('should not set Content-Type for GET requests', async () => {
      mockResponse(200, {})

      await get('/api/v1/files')

      const [, options] = mockFetch.mock.calls[0]
      expect(options.headers['Content-Type']).toBeUndefined()
    })
  })

  describe('post', () => {
    it('should make a POST request with JSON body', async () => {
      mockResponse(200, { content: 'generated' })

      const result = await post<{ content: string }>('/api/v1/import/curl', { command: 'curl http://example.com' })

      expect(result).toEqual({ content: 'generated' })
      const [, options] = mockFetch.mock.calls[0]
      expect(options.method).toBe('POST')
      expect(options.headers['Content-Type']).toBe('application/json')
      expect(JSON.parse(options.body)).toEqual({ command: 'curl http://example.com' })
    })

    it('should handle POST without body', async () => {
      mockResponse(200, {})

      await post('/api/v1/stress/stop')

      const [, options] = mockFetch.mock.calls[0]
      expect(options.body).toBeUndefined()
    })
  })

  describe('put', () => {
    it('should make a PUT request', async () => {
      mockResponse(200, {})

      await put('/api/v1/config', { timeout: 5000 })

      const [, options] = mockFetch.mock.calls[0]
      expect(options.method).toBe('PUT')
    })
  })

  describe('del', () => {
    it('should make a DELETE request', async () => {
      mockResponse(204, undefined)

      await del('/api/v1/history')

      const [, options] = mockFetch.mock.calls[0]
      expect(options.method).toBe('DELETE')
    })
  })

  describe('error handling', () => {
    it('should throw ApiError on non-ok response', async () => {
      mockResponse(404, { error: 'Not found' }, false)

      await expect(get('/api/v1/files/missing')).rejects.toThrow(ApiError)
    })

    it('should include status and body in ApiError', async () => {
      mockResponse(400, { error: 'Bad request', message: 'missing field' }, false)

      try {
        await post('/api/v1/execute', {})
        expect.fail('should have thrown')
      } catch (e) {
        expect(e).toBeInstanceOf(ApiError)
        const err = e as ApiError
        expect(err.status).toBe(400)
        expect(err.body).toEqual({ error: 'Bad request', message: 'missing field' })
      }
    })

    it('should handle text error body', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 500,
        statusText: 'Internal Server Error',
        json: () => Promise.reject(new Error('not json')),
        text: () => Promise.resolve('server error text'),
      })

      try {
        await get('/api/v1/crash')
        expect.fail('should have thrown')
      } catch (e) {
        const err = e as ApiError
        expect(err.status).toBe(500)
        expect(err.body).toBe('server error text')
      }
    })

    it('should return undefined for 204 No Content', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 204,
        statusText: 'No Content',
        json: () => Promise.reject(new Error('no body')),
      })

      const result = await del('/api/v1/history')
      expect(result).toBeUndefined()
    })
  })
})
