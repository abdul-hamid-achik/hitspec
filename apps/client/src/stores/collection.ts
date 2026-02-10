import { ref, shallowRef, computed, triggerRef } from 'vue'
import { defineStore } from 'pinia'
import type { FileInfo, ParsedFile } from '@/types/api'
import { getWorkspace, getFile } from '@/api/endpoints/files'
import { ws } from '@/api/websocket'

export const useCollectionStore = defineStore('collection', () => {
  const files = ref<FileInfo[]>([])
  const openFiles = shallowRef<Map<string, ParsedFile>>(new Map())
  const activeFilePath = ref<string | null>(null)
  const expandedFiles = shallowRef<Set<string>>(new Set())
  const loading = ref(false)
  const error = ref<string | null>(null)

  // Prevent concurrent openFile() calls from clobbering each other
  const pendingOpens = new Set<string>()

  const activeFile = computed(() => {
    if (!activeFilePath.value) return null
    return openFiles.value.get(activeFilePath.value) ?? null
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
        const parsed = await getFile(path)
        openFiles.value.set(path, parsed)
        triggerRef(openFiles)
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
    triggerRef(openFiles)
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
        // Refresh all open files, not just the active one
        for (const path of openFiles.value.keys()) {
          getFile(path).then((parsed) => {
            openFiles.value.set(path, parsed)
            triggerRef(openFiles)
          }).catch(() => {
            // File may have been deleted; remove from open files
            openFiles.value.delete(path)
            triggerRef(openFiles)
            if (activeFilePath.value === path) {
              const remaining = [...openFiles.value.keys()]
              activeFilePath.value = remaining.length > 0 ? remaining[remaining.length - 1] : null
            }
          })
        }
      }, 300)
    })
  }

  return { files, openFiles, activeFilePath, expandedFiles, activeFile, fileCount, workspaceEnvironment, loading, error, loadFiles, openFile, closeFile, toggleFileExpanded, init }
})
