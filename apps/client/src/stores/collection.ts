import { ref, shallowRef, computed, triggerRef } from 'vue'
import { defineStore } from 'pinia'
import type { FileInfo, ParsedFile } from '@/types/api'
import { getWorkspace, getFile, getFileRaw, saveFile as apiSaveFile, createFile as apiCreateFile, deleteFile as apiDeleteFile } from '@/api/endpoints/files'
import { ws } from '@/api/websocket'

export const useCollectionStore = defineStore('collection', () => {
  const files = ref<FileInfo[]>([])
  const openFiles = shallowRef<Map<string, ParsedFile>>(new Map())
  const activeFilePath = ref<string | null>(null)
  const expandedFiles = shallowRef<Set<string>>(new Set())
  const loading = ref(false)
  const error = ref<string | null>(null)

  // Raw file content for the source editor
  const rawContents = shallowRef<Map<string, string>>(new Map())
  // Dirty state: tracks which files have unsaved edits
  const dirtyFiles = shallowRef<Set<string>>(new Set())
  // Saving state
  const saving = ref(false)

  // Prevent concurrent openFile() calls from clobbering each other
  const pendingOpens = new Set<string>()
  // Tracks paths recently saved by this client to suppress redundant re-fetch
  const recentlySaved = new Set<string>()

  const activeFile = computed(() => {
    if (!activeFilePath.value) return null
    return openFiles.value.get(activeFilePath.value) ?? null
  })

  const activeRawContent = computed(() => {
    if (!activeFilePath.value) return null
    return rawContents.value.get(activeFilePath.value) ?? null
  })

  const isActiveDirty = computed(() => {
    if (!activeFilePath.value) return false
    return dirtyFiles.value.has(activeFilePath.value)
  })

  const fileCount = computed(() => {
    let count = 0
    const walk = (items: FileInfo[]) => {
      for (const item of items) {
        if (!item.isDir) count++
        if (item.children) walk(item.children)
      }
    }
    walk(files.value)
    return count
  })

  function isFileDirty(path: string): boolean {
    return dirtyFiles.value.has(path)
  }

  // Exposes the workspace environment name so callers can seed the env store
  const workspaceEnvironment = ref('')

  async function loadFiles() {
    loading.value = true
    error.value = null
    try {
      const workspace = await getWorkspace()
      files.value = workspace.files
      if (workspace.environment) {
        workspaceEnvironment.value = workspace.environment
      }
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to load workspace'
    } finally {
      loading.value = false
    }
  }

  async function openFile(path: string) {
    if (!openFiles.value.has(path)) {
      if (pendingOpens.has(path)) return // already loading this file
      pendingOpens.add(path)
      try {
        const [parsed, raw] = await Promise.all([getFile(path), getFileRaw(path)])
        openFiles.value.set(path, parsed)
        rawContents.value.set(path, raw)
        triggerRef(openFiles)
        triggerRef(rawContents)
      } catch (e) {
        error.value = e instanceof Error ? e.message : `Failed to open ${path}`
        return
      } finally {
        pendingOpens.delete(path)
      }
    }
    activeFilePath.value = path
    expandedFiles.value.add(path)
    triggerRef(expandedFiles)
  }

  function updateRawContent(path: string, content: string) {
    rawContents.value.set(path, content)
    dirtyFiles.value.add(path)
    triggerRef(rawContents)
    triggerRef(dirtyFiles)
  }

  async function saveActiveFile() {
    const path = activeFilePath.value
    if (!path) return
    const content = rawContents.value.get(path)
    if (content === undefined) return

    saving.value = true
    error.value = null
    try {
      const parsed = await apiSaveFile(path, content)
      openFiles.value.set(path, parsed)
      dirtyFiles.value.delete(path)
      // Suppress the upcoming file_changed WS event for this path
      recentlySaved.add(path)
      setTimeout(() => recentlySaved.delete(path), 2000)
      triggerRef(openFiles)
      triggerRef(dirtyFiles)
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to save file'
    } finally {
      saving.value = false
    }
  }

  async function saveAllDirtyFiles() {
    const paths = [...dirtyFiles.value]
    if (paths.length === 0) return
    saving.value = true
    error.value = null
    const errors: string[] = []
    for (const path of paths) {
      const content = rawContents.value.get(path)
      if (content === undefined) continue
      try {
        const parsed = await apiSaveFile(path, content)
        openFiles.value.set(path, parsed)
        dirtyFiles.value.delete(path)
        recentlySaved.add(path)
        setTimeout(() => recentlySaved.delete(path), 2000)
      } catch (e) {
        errors.push(`${path}: ${e instanceof Error ? e.message : 'Failed to save'}`)
      }
    }
    triggerRef(openFiles)
    triggerRef(dirtyFiles)
    saving.value = false
    if (errors.length > 0) {
      error.value = errors.join('; ')
    }
  }

  async function createNewFile(path: string, content?: string) {
    error.value = null
    try {
      await apiCreateFile(path, content)
      await loadFiles()
      await openFile(path)
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to create file'
    }
  }

  async function deleteCurrentFile(path: string) {
    error.value = null
    try {
      await apiDeleteFile(path)
      closeFile(path)
      await loadFiles()
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to delete file'
    }
  }

  function toggleFileExpanded(path: string) {
    if (expandedFiles.value.has(path)) {
      expandedFiles.value.delete(path)
    } else {
      expandedFiles.value.add(path)
    }
    triggerRef(expandedFiles)
  }

  function closeFile(path: string) {
    openFiles.value.delete(path)
    rawContents.value.delete(path)
    dirtyFiles.value.delete(path)
    triggerRef(openFiles)
    triggerRef(rawContents)
    triggerRef(dirtyFiles)
    if (activeFilePath.value === path) {
      const paths = [...openFiles.value.keys()]
      activeFilePath.value = paths.length > 0 ? paths[paths.length - 1] : null
    }
  }

  let initialized = false
  let fileChangeTimer: ReturnType<typeof setTimeout> | null = null

  function init() {
    if (initialized) return
    initialized = true
    ws.on('file_changed', () => {
      // Debounce rapid file-change events (editors often trigger multiple saves)
      if (fileChangeTimer) clearTimeout(fileChangeTimer)
      fileChangeTimer = setTimeout(() => {
        fileChangeTimer = null
        loadFiles()
        // Refresh all open files that are NOT dirty and not recently saved by us
        for (const path of openFiles.value.keys()) {
          if (dirtyFiles.value.has(path) || recentlySaved.has(path)) continue
          Promise.all([getFile(path), getFileRaw(path)]).then(([parsed, raw]) => {
            openFiles.value.set(path, parsed)
            rawContents.value.set(path, raw)
            triggerRef(openFiles)
            triggerRef(rawContents)
          }).catch(() => {
            // File may have been deleted; remove from open files
            openFiles.value.delete(path)
            rawContents.value.delete(path)
            dirtyFiles.value.delete(path)
            triggerRef(openFiles)
            triggerRef(rawContents)
            triggerRef(dirtyFiles)
            if (activeFilePath.value === path) {
              const remaining = [...openFiles.value.keys()]
              activeFilePath.value = remaining.length > 0 ? remaining[remaining.length - 1] : null
            }
          })
        }
      }, 300)
    })
  }

  return {
    files, openFiles, activeFilePath, expandedFiles, activeFile, activeRawContent,
    isActiveDirty, fileCount, workspaceEnvironment, loading, error, saving, dirtyFiles,
    loadFiles, openFile, closeFile, toggleFileExpanded, init,
    updateRawContent, saveActiveFile, saveAllDirtyFiles, createNewFile, deleteCurrentFile, isFileDirty,
  }
})
