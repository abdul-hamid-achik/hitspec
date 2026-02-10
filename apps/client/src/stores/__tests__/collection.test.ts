import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useCollectionStore } from '@/stores/collection'
import type { WorkspaceInfo, ParsedFile } from '@/types/api'

vi.mock('@/api/endpoints/files', () => ({
  getWorkspace: vi.fn(),
  getFile: vi.fn(),
}))

vi.mock('@/api/websocket', () => ({
  ws: { on: vi.fn() },
}))

import { getWorkspace, getFile } from '@/api/endpoints/files'
import { ws } from '@/api/websocket'

const mockWorkspace: WorkspaceInfo = {
  root: '/project',
  files: [
    { path: 'api.http', name: 'api.http', dir: '/', isDir: false },
    {
      path: 'tests',
      name: 'tests',
      dir: '/',
      isDir: true,
      children: [
        { path: 'tests/auth.http', name: 'auth.http', dir: '/tests', isDir: false },
        { path: 'tests/users.http', name: 'users.http', dir: '/tests', isDir: false },
      ],
    },
  ],
  totalRequests: 10,
  environment: 'dev',
  hasConfig: true,
}

const mockParsed: ParsedFile = {
  path: 'api.http',
  variables: [{ name: 'base', value: 'http://localhost', line: 1 }],
  requests: [
    { name: 'Get users', method: 'GET', url: '{{base}}/users', line: 3 },
  ],
}

describe('Collection Store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.mocked(getWorkspace).mockReset()
    vi.mocked(getFile).mockReset()
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  describe('initial state', () => {
    it('should have empty defaults', () => {
      const store = useCollectionStore()

      expect(store.files).toEqual([])
      expect(store.openFiles.size).toBe(0)
      expect(store.activeFilePath).toBeNull()
      expect(store.expandedFiles.size).toBe(0)
      expect(store.loading).toBe(false)
      expect(store.error).toBeNull()
      expect(store.workspaceEnvironment).toBe('')
    })
  })

  describe('computed: activeFile', () => {
    it('should return null when no active file path', () => {
      const store = useCollectionStore()
      expect(store.activeFile).toBeNull()
    })

    it('should return null when active path is not in openFiles', () => {
      const store = useCollectionStore()
      store.activeFilePath = 'missing.http'
      expect(store.activeFile).toBeNull()
    })

    it('should return the parsed file when active path matches', async () => {
      vi.mocked(getFile).mockResolvedValueOnce(mockParsed)
      const store = useCollectionStore()

      await store.openFile('api.http')

      expect(store.activeFile).toEqual(mockParsed)
    })
  })

  describe('computed: fileCount', () => {
    it('should return 0 for empty files', () => {
      const store = useCollectionStore()
      expect(store.fileCount).toBe(0)
    })

    it('should count only non-directory files recursively', async () => {
      vi.mocked(getWorkspace).mockResolvedValueOnce(mockWorkspace)
      const store = useCollectionStore()

      await store.loadFiles()

      // 1 file at root + 2 files in tests/ dir = 3
      expect(store.fileCount).toBe(3)
    })
  })

  describe('loadFiles', () => {
    it('should load workspace files and set environment', async () => {
      vi.mocked(getWorkspace).mockResolvedValueOnce(mockWorkspace)
      const store = useCollectionStore()

      await store.loadFiles()

      expect(store.files).toEqual(mockWorkspace.files)
      expect(store.workspaceEnvironment).toBe('dev')
      expect(store.loading).toBe(false)
      expect(store.error).toBeNull()
    })

    it('should set loading to true during fetch', async () => {
      let resolvePromise!: (value: WorkspaceInfo) => void
      vi.mocked(getWorkspace).mockReturnValueOnce(
        new Promise((resolve) => { resolvePromise = resolve }),
      )
      const store = useCollectionStore()

      const promise = store.loadFiles()
      expect(store.loading).toBe(true)

      resolvePromise(mockWorkspace)
      await promise
      expect(store.loading).toBe(false)
    })

    it('should set error on failure', async () => {
      vi.mocked(getWorkspace).mockRejectedValueOnce(new Error('Network error'))
      const store = useCollectionStore()

      await store.loadFiles()

      expect(store.error).toBe('Network error')
      expect(store.loading).toBe(false)
    })

    it('should set generic error for non-Error throws', async () => {
      vi.mocked(getWorkspace).mockRejectedValueOnce('timeout')
      const store = useCollectionStore()

      await store.loadFiles()

      expect(store.error).toBe('Failed to load workspace')
    })

    it('should not set environment when workspace has no environment', async () => {
      vi.mocked(getWorkspace).mockResolvedValueOnce({
        ...mockWorkspace,
        environment: '',
      })
      const store = useCollectionStore()

      await store.loadFiles()

      expect(store.workspaceEnvironment).toBe('')
    })
  })

  describe('openFile', () => {
    it('should fetch and open a file', async () => {
      vi.mocked(getFile).mockResolvedValueOnce(mockParsed)
      const store = useCollectionStore()

      await store.openFile('api.http')

      expect(getFile).toHaveBeenCalledWith('api.http')
      expect(store.openFiles.has('api.http')).toBe(true)
      expect(store.activeFilePath).toBe('api.http')
      expect(store.expandedFiles.has('api.http')).toBe(true)
    })

    it('should not refetch an already-open file', async () => {
      vi.mocked(getFile).mockResolvedValueOnce(mockParsed)
      const store = useCollectionStore()

      await store.openFile('api.http')
      await store.openFile('api.http')

      expect(getFile).toHaveBeenCalledTimes(1)
      expect(store.activeFilePath).toBe('api.http')
    })

    it('should set error on fetch failure', async () => {
      vi.mocked(getFile).mockRejectedValueOnce(new Error('Not found'))
      const store = useCollectionStore()

      await store.openFile('missing.http')

      expect(store.error).toBe('Not found')
      expect(store.openFiles.has('missing.http')).toBe(false)
    })

    it('should not set activeFilePath on fetch failure', async () => {
      vi.mocked(getFile).mockRejectedValueOnce(new Error('fail'))
      const store = useCollectionStore()

      await store.openFile('broken.http')

      expect(store.activeFilePath).toBeNull()
    })

    it('should guard against concurrent opens of the same file', async () => {
      let resolveFirst!: (value: ParsedFile) => void
      vi.mocked(getFile).mockReturnValueOnce(
        new Promise((resolve) => { resolveFirst = resolve }),
      )
      const store = useCollectionStore()

      const p1 = store.openFile('api.http')
      const p2 = store.openFile('api.http') // should be a no-op

      resolveFirst(mockParsed)
      await Promise.all([p1, p2])

      expect(getFile).toHaveBeenCalledTimes(1)
    })
  })

  describe('closeFile', () => {
    it('should remove a file from openFiles', async () => {
      vi.mocked(getFile).mockResolvedValueOnce(mockParsed)
      const store = useCollectionStore()

      await store.openFile('api.http')
      store.closeFile('api.http')

      expect(store.openFiles.has('api.http')).toBe(false)
    })

    it('should switch activeFilePath to last remaining open file', async () => {
      const parsed2: ParsedFile = { path: 'b.http', variables: [], requests: [] }
      vi.mocked(getFile)
        .mockResolvedValueOnce(mockParsed)
        .mockResolvedValueOnce(parsed2)
      const store = useCollectionStore()

      await store.openFile('api.http')
      await store.openFile('b.http')
      store.closeFile('b.http')

      expect(store.activeFilePath).toBe('api.http')
    })

    it('should set activeFilePath to null when last file is closed', async () => {
      vi.mocked(getFile).mockResolvedValueOnce(mockParsed)
      const store = useCollectionStore()

      await store.openFile('api.http')
      store.closeFile('api.http')

      expect(store.activeFilePath).toBeNull()
    })

    it('should not change activeFilePath when closing a non-active file', async () => {
      const parsed2: ParsedFile = { path: 'b.http', variables: [], requests: [] }
      vi.mocked(getFile)
        .mockResolvedValueOnce(mockParsed)
        .mockResolvedValueOnce(parsed2)
      const store = useCollectionStore()

      await store.openFile('api.http')
      await store.openFile('b.http')

      // active is now b.http, close api.http
      store.closeFile('api.http')
      expect(store.activeFilePath).toBe('b.http')
    })
  })

  describe('toggleFileExpanded', () => {
    it('should add path to expandedFiles', () => {
      const store = useCollectionStore()

      store.toggleFileExpanded('tests')

      expect(store.expandedFiles.has('tests')).toBe(true)
    })

    it('should remove path from expandedFiles on second toggle', () => {
      const store = useCollectionStore()

      store.toggleFileExpanded('tests')
      store.toggleFileExpanded('tests')

      expect(store.expandedFiles.has('tests')).toBe(false)
    })
  })

  describe('init', () => {
    it('should register a file_changed listener on ws', () => {
      const store = useCollectionStore()

      store.init()

      expect(ws.on).toHaveBeenCalledWith('file_changed', expect.any(Function))
    })

    it('should only register listener once', () => {
      const store = useCollectionStore()

      store.init()
      store.init()

      // The mock tracks calls across the store's lifetime; init guards with `initialized`
      expect(vi.mocked(ws.on)).toHaveBeenCalledTimes(1)
    })
  })
})
