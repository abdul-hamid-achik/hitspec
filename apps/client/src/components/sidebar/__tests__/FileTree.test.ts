import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { shallowMount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import type { Pinia } from 'pinia'
import FileTree from '@/components/sidebar/FileTree.vue'
import { useCollectionStore } from '@/stores/collection'
import { useRequestStore } from '@/stores/request'
import { ChevronRight, FileText, Folder, FolderOpen } from 'lucide-vue-next'
import MethodBadge from '@/components/common/MethodBadge.vue'
import type { FileInfo, RequestDTO, ParsedFile } from '@/types/api'

// Mock API modules that the stores and component import
vi.mock('@/api/endpoints/files', () => ({
  getWorkspace: vi.fn(),
  getFile: vi.fn(),
}))
vi.mock('@/api/endpoints/execute', () => ({
  executeRequest: vi.fn(),
  executeFile: vi.fn(),
}))
vi.mock('@/api/endpoints/environments', () => ({
  getEnvironments: vi.fn().mockResolvedValue([]),
  selectEnvironment: vi.fn(),
}))
vi.mock('@/api/websocket', () => ({
  ws: { on: vi.fn(), off: vi.fn(), send: vi.fn() },
}))

function makeFileItem(overrides: Partial<FileInfo> = {}): FileInfo {
  return {
    path: '/workspace/test.http',
    name: 'test.http',
    dir: '/workspace',
    isDir: false,
    requestCount: 2,
    ...overrides,
  }
}

function makeDirItem(overrides: Partial<FileInfo> = {}): FileInfo {
  return {
    path: '/workspace/api',
    name: 'api',
    dir: '/workspace',
    isDir: true,
    children: [
      makeFileItem({ path: '/workspace/api/users.http', name: 'users.http', dir: '/workspace/api' }),
    ],
    ...overrides,
  }
}

function makeRequest(name: string): RequestDTO {
  return {
    name,
    method: 'GET',
    url: `https://api.example.com/${name.toLowerCase().replace(/\s+/g, '-')}`,
    line: 1,
  }
}

function makeParsedFile(path: string, requests: RequestDTO[]): ParsedFile {
  return { path, variables: [], requests }
}

describe('FileTree', () => {
  let pinia: Pinia

  beforeEach(() => {
    pinia = createPinia()
    setActivePinia(pinia)
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  function mountTree(items: FileInfo[], depth = 0) {
    return shallowMount(FileTree, {
      props: { items, depth },
      global: { plugins: [pinia] },
    })
  }

  it('should render file items with FileText icon', () => {
    const items = [makeFileItem()]
    const wrapper = mountTree(items)

    expect(wrapper.findComponent(FileText).exists()).toBe(true)
    expect(wrapper.text()).toContain('test.http')
  })

  it('should render directory items with Folder icon', () => {
    const items = [makeDirItem()]
    const wrapper = mountTree(items)

    expect(wrapper.findComponent(Folder).exists()).toBe(true)
    expect(wrapper.text()).toContain('api')
  })

  it('should render chevron for directories', () => {
    const items = [makeDirItem()]
    const wrapper = mountTree(items)

    expect(wrapper.findComponent(ChevronRight).exists()).toBe(true)
  })

  it('should render chevron for files with requests', () => {
    const items = [makeFileItem({ requestCount: 3 })]
    const wrapper = mountTree(items)

    expect(wrapper.findComponent(ChevronRight).exists()).toBe(true)
  })

  it('should not render chevron for files without requests', () => {
    const items = [makeFileItem({ requestCount: 0 })]
    const wrapper = mountTree(items)

    expect(wrapper.findComponent(ChevronRight).exists()).toBe(false)
  })

  it('should show request count badge for files', () => {
    const items = [makeFileItem({ requestCount: 5 })]
    const wrapper = mountTree(items)

    expect(wrapper.text()).toContain('5')
  })

  it('should toggle directory expansion on click', async () => {
    const items = [makeDirItem()]
    const wrapper = mountTree(items)

    // Initially collapsed - Folder icon shown (not FolderOpen)
    expect(wrapper.findComponent(Folder).exists()).toBe(true)
    expect(wrapper.findComponent(FolderOpen).exists()).toBe(false)

    // Click the directory tree item
    const treeItems = wrapper.findAll('[role="treeitem"]')
    const dirItem = treeItems.find((el) => el.text().includes('api'))
    await dirItem!.trigger('click')

    // Now expanded - FolderOpen icon shown
    expect(wrapper.findComponent(FolderOpen).exists()).toBe(true)

    // Click again to collapse
    await dirItem!.trigger('click')

    expect(wrapper.findComponent(Folder).exists()).toBe(true)
    expect(wrapper.findComponent(FolderOpen).exists()).toBe(false)
  })

  it('should show Play button for directories', () => {
    const items = [makeDirItem()]
    const wrapper = mountTree(items)

    const playButtons = wrapper.findAll('button[aria-label="Run all files in folder"]')
    expect(playButtons.length).toBe(1)
  })

  it('should show Play button for files with requests', () => {
    const items = [makeFileItem({ requestCount: 2 })]
    const wrapper = mountTree(items)

    const playButtons = wrapper.findAll('button[aria-label="Run file"]')
    expect(playButtons.length).toBe(1)
  })

  it('should highlight the active file', () => {
    const collection = useCollectionStore()
    const items = [makeFileItem()]
    collection.activeFilePath = '/workspace/test.http'

    const wrapper = mountTree(items)

    const treeItems = wrapper.findAll('[role="treeitem"]')
    const fileItem = treeItems.find((el) => el.text().includes('test.http'))
    expect(fileItem!.classes()).toContain('bg-accent/10')
  })

  it('should show request sub-list when file is expanded', () => {
    const collection = useCollectionStore()
    const items = [makeFileItem()]

    // Set up the file as open and expanded
    const parsedFile = makeParsedFile('/workspace/test.http', [
      makeRequest('Get Users'),
      makeRequest('Create User'),
    ])
    collection.openFiles.set('/workspace/test.http', parsedFile)
    collection.expandedFiles.add('/workspace/test.http')

    const wrapper = mountTree(items)

    // Should render MethodBadge for each request
    const badges = wrapper.findAllComponents(MethodBadge)
    expect(badges.length).toBe(2)

    // Should show request names/URLs
    expect(wrapper.text()).toContain('Get Users')
    expect(wrapper.text()).toContain('Create User')
  })

  it('should call setActiveRequest when request sub-item is clicked', async () => {
    const collection = useCollectionStore()
    const requestStore = useRequestStore()
    const setActiveSpy = vi.spyOn(requestStore, 'setActiveRequest')

    const items = [makeFileItem()]
    const req1 = makeRequest('Get Users')
    const req2 = makeRequest('Create User')
    const parsedFile = makeParsedFile('/workspace/test.http', [req1, req2])
    collection.openFiles.set('/workspace/test.http', parsedFile)
    collection.expandedFiles.add('/workspace/test.http')

    const wrapper = mountTree(items)

    // Click the second request
    const requestButtons = wrapper.findAll('button').filter((b) => b.text().includes('Create User'))
    await requestButtons[0].trigger('click')

    expect(setActiveSpy).toHaveBeenCalledWith(req2, 1)
    expect(collection.activeFilePath).toBe('/workspace/test.http')
  })

  it('should set activeRequest in store when request sub-item is clicked', async () => {
    const collection = useCollectionStore()
    const requestStore = useRequestStore()
    const items = [makeFileItem()]

    const req1 = makeRequest('Get Users')
    const req2 = makeRequest('Create User')
    const parsedFile = makeParsedFile('/workspace/test.http', [req1, req2])
    collection.openFiles.set('/workspace/test.http', parsedFile)
    collection.expandedFiles.add('/workspace/test.http')

    const wrapper = mountTree(items)

    // Click on "Get Users" request
    const requestButtons = wrapper.findAll('button').filter((b) => b.text().includes('Get Users'))
    await requestButtons[0].trigger('click')

    // Verify the store is updated correctly
    expect(requestStore.activeRequest).toStrictEqual(req1)
    expect(requestStore.activeRequestIndex).toBe(0)
    expect(collection.activeFilePath).toBe('/workspace/test.http')

    // Click on "Create User" request
    const createButtons = wrapper.findAll('button').filter((b) => b.text().includes('Create User'))
    await createButtons[0].trigger('click')

    expect(requestStore.activeRequest).toStrictEqual(req2)
    expect(requestStore.activeRequestIndex).toBe(1)
  })

  it('should render multiple items', () => {
    const items = [
      makeFileItem({ path: '/workspace/a.http', name: 'a.http' }),
      makeFileItem({ path: '/workspace/b.http', name: 'b.http' }),
      makeDirItem(),
    ]
    const wrapper = mountTree(items)

    expect(wrapper.text()).toContain('a.http')
    expect(wrapper.text()).toContain('b.http')
    expect(wrapper.text()).toContain('api')
  })

  it('should show export button for request sub-items', () => {
    const collection = useCollectionStore()
    const items = [makeFileItem()]
    const parsedFile = makeParsedFile('/workspace/test.http', [makeRequest('Get Users')])
    collection.openFiles.set('/workspace/test.http', parsedFile)
    collection.expandedFiles.add('/workspace/test.http')

    const wrapper = mountTree(items)

    const exportButtons = wrapper.findAll('button[aria-label="Export request"]')
    expect(exportButtons.length).toBe(1)
  })
})
