import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useRequestStore } from '@/stores/request'
import type { ExecuteResult, RunResult, RequestDTO, WSRequestProgress } from '@/types/api'

vi.mock('@/api/endpoints/execute', () => ({
  executeRequest: vi.fn(),
  executeFile: vi.fn(),
}))

import { executeRequest as apiExecute, executeFile as executeFileApi } from '@/api/endpoints/execute'

const makeResult = (name: string, passed = true): RunResult => ({
  name,
  passed,
  duration: 120,
  response: {
    statusCode: 200,
    status: 'OK',
    headers: {},
    body: '{}',
    duration: 100,
    size: 2,
  },
})

const makeExecResult = (results: RunResult[]): ExecuteResult => ({
  file: 'api.http',
  duration: 500,
  passed: results.filter((r) => r.passed).length,
  failed: results.filter((r) => !r.passed).length,
  skipped: 0,
  results,
})

describe('Request Store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.mocked(apiExecute).mockReset()
    vi.mocked(executeFileApi).mockReset()
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  describe('initial state', () => {
    it('should have correct defaults', () => {
      const store = useRequestStore()

      expect(store.activeRequest).toBeNull()
      expect(store.activeRequestIndex).toBe(0)
      expect(store.lastResult).toBeNull()
      expect(store.lastRunResult).toBeNull()
      expect(store.isExecuting).toBe(false)
      expect(store.error).toBeNull()
      expect(store.executionProgress).toBeNull()
    })
  })

  describe('execute', () => {
    it('should execute a single request and store the result', async () => {
      const result = makeExecResult([makeResult('Get users')])
      vi.mocked(apiExecute).mockResolvedValueOnce(result)
      const store = useRequestStore()

      await store.execute('api.http', 'Get users', 'dev')

      expect(apiExecute).toHaveBeenCalledWith({
        file: 'api.http',
        requestName: 'Get users',
        environment: 'dev',
      })
      expect(store.lastResult).toEqual(result.results[0])
      expect(store.isExecuting).toBe(false)
      expect(store.error).toBeNull()
    })

    it('should set isExecuting during request', async () => {
      let resolvePromise!: (value: ExecuteResult) => void
      vi.mocked(apiExecute).mockReturnValueOnce(
        new Promise((resolve) => { resolvePromise = resolve }),
      )
      const store = useRequestStore()

      const promise = store.execute('api.http', 'test')
      expect(store.isExecuting).toBe(true)

      resolvePromise(makeExecResult([makeResult('test')]))
      await promise
      expect(store.isExecuting).toBe(false)
    })

    it('should find matching result by requestName', async () => {
      const results = [makeResult('First'), makeResult('Second')]
      vi.mocked(apiExecute).mockResolvedValueOnce(makeExecResult(results))
      const store = useRequestStore()

      await store.execute('api.http', 'Second')

      expect(store.lastResult?.name).toBe('Second')
    })

    it('should fall back to first result if requestName not found', async () => {
      const results = [makeResult('First')]
      vi.mocked(apiExecute).mockResolvedValueOnce(makeExecResult(results))
      const store = useRequestStore()

      await store.execute('api.http', 'Missing')

      expect(store.lastResult?.name).toBe('First')
    })

    it('should use first result when no requestName provided', async () => {
      const results = [makeResult('First'), makeResult('Second')]
      vi.mocked(apiExecute).mockResolvedValueOnce(makeExecResult(results))
      const store = useRequestStore()

      await store.execute('api.http')

      expect(store.lastResult?.name).toBe('First')
    })

    it('should set error on failure', async () => {
      vi.mocked(apiExecute).mockRejectedValueOnce(new Error('Connection refused'))
      const store = useRequestStore()

      await store.execute('api.http', 'test')

      expect(store.error).toBe('Connection refused')
      expect(store.isExecuting).toBe(false)
    })

    it('should not allow concurrent executions', async () => {
      let resolveFirst!: (value: ExecuteResult) => void
      vi.mocked(apiExecute).mockReturnValueOnce(
        new Promise((resolve) => { resolveFirst = resolve }),
      )
      const store = useRequestStore()

      const p1 = store.execute('api.http', 'a')
      store.execute('api.http', 'b') // should be no-op

      expect(apiExecute).toHaveBeenCalledTimes(1)

      resolveFirst(makeExecResult([makeResult('a')]))
      await p1
    })

    it('should clear previous state before executing', async () => {
      vi.mocked(apiExecute).mockResolvedValueOnce(makeExecResult([makeResult('first')]))
      const store = useRequestStore()

      await store.execute('api.http', 'first')
      expect(store.lastResult).not.toBeNull()

      vi.mocked(apiExecute).mockResolvedValueOnce(makeExecResult([makeResult('second')]))
      await store.execute('api.http', 'second')

      expect(store.lastResult?.name).toBe('second')
    })
  })

  describe('runFile', () => {
    it('should execute all requests in a file', async () => {
      const results = [makeResult('First'), makeResult('Second')]
      const execResult = makeExecResult(results)
      vi.mocked(executeFileApi).mockResolvedValueOnce(execResult)
      const store = useRequestStore()

      await store.runFile('api.http', 'dev')

      expect(executeFileApi).toHaveBeenCalledWith('api.http', 'dev')
      expect(store.lastRunResult).toEqual(execResult)
      expect(store.lastResult).toEqual(results[0])
    })

    it('should set error on failure', async () => {
      vi.mocked(executeFileApi).mockRejectedValueOnce(new Error('Timeout'))
      const store = useRequestStore()

      await store.runFile('api.http')

      expect(store.error).toBe('Timeout')
      expect(store.isExecuting).toBe(false)
    })

    it('should not run if already executing', async () => {
      let resolveFirst!: (value: ExecuteResult) => void
      vi.mocked(executeFileApi).mockReturnValueOnce(
        new Promise((resolve) => { resolveFirst = resolve }),
      )
      const store = useRequestStore()

      const p1 = store.runFile('api.http')
      store.runFile('api.http') // no-op

      expect(executeFileApi).toHaveBeenCalledTimes(1)

      resolveFirst(makeExecResult([]))
      await p1
    })
  })

  describe('setActiveRequest', () => {
    it('should set activeRequest and index', () => {
      const store = useRequestStore()
      const req: RequestDTO = {
        name: 'Get users',
        method: 'GET',
        url: '/users',
        line: 1,
      }

      store.setActiveRequest(req, 2)

      expect(store.activeRequest).toEqual(req)
      expect(store.activeRequestIndex).toBe(2)
    })

    it('should clear lastResult when switching request', () => {
      const store = useRequestStore()
      store.lastResult = makeResult('old')

      store.setActiveRequest(
        { name: 'New', method: 'POST', url: '/new', line: 5 },
        0,
      )

      expect(store.lastResult).toBeNull()
      expect(store.error).toBeNull()
    })

    it('should match lastRunResult entry when available', async () => {
      const results = [makeResult('A'), makeResult('B')]
      vi.mocked(executeFileApi).mockResolvedValueOnce(makeExecResult(results))
      const store = useRequestStore()

      await store.runFile('api.http')

      store.setActiveRequest({ name: 'B', method: 'GET', url: '/b', line: 10 })

      expect(store.lastResult?.name).toBe('B')
    })

    it('should set null for request=null', () => {
      const store = useRequestStore()
      store.activeRequest = { name: 'X', method: 'GET', url: '/x', line: 1 } as RequestDTO

      store.setActiveRequest(null)

      expect(store.activeRequest).toBeNull()
      expect(store.activeRequestIndex).toBe(0)
    })
  })

  describe('handleProgress', () => {
    it('should handle started progress event', () => {
      const store = useRequestStore()
      store.isExecuting = true

      const progress: WSRequestProgress = {
        execId: '1',
        file: 'api.http',
        requestName: 'Get users',
        status: 'started',
        index: 0,
        total: 3,
        timestamp: new Date().toISOString(),
      }

      store.handleProgress(progress)

      expect(store.executionProgress).toEqual({
        currentRequest: 'Get users',
        index: 0,
        total: 3,
        completed: 0,
        results: [],
      })
    })

    it('should handle completed progress event', () => {
      const store = useRequestStore()
      store.isExecuting = true

      // First, start
      store.handleProgress({
        execId: '1',
        file: 'api.http',
        requestName: 'Get users',
        status: 'started',
        index: 0,
        total: 2,
        timestamp: new Date().toISOString(),
      })

      // Then, complete
      store.handleProgress({
        execId: '1',
        file: 'api.http',
        requestName: 'Get users',
        status: 'completed',
        index: 0,
        total: 2,
        passed: true,
        duration: 150,
        timestamp: new Date().toISOString(),
      })

      expect(store.executionProgress?.completed).toBe(1)
      expect(store.executionProgress?.results).toHaveLength(1)
      expect(store.executionProgress?.results[0]).toEqual({
        name: 'Get users',
        passed: true,
        duration: 150,
      })
    })

    it('should ignore progress when not executing', () => {
      const store = useRequestStore()
      store.isExecuting = false

      store.handleProgress({
        execId: '1',
        file: 'api.http',
        requestName: 'test',
        status: 'started',
        index: 0,
        total: 1,
        timestamp: new Date().toISOString(),
      })

      expect(store.executionProgress).toBeNull()
    })

    it('should use fallback name when requestName is empty', () => {
      const store = useRequestStore()
      store.isExecuting = true

      store.handleProgress({
        execId: '1',
        file: 'api.http',
        requestName: '',
        status: 'started',
        index: 2,
        total: 5,
        timestamp: new Date().toISOString(),
      })

      expect(store.executionProgress?.currentRequest).toBe('Request 3')
    })
  })
})
