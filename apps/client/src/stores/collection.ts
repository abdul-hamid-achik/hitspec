import { ref, computed } from 'vue'
import { defineStore } from 'pinia'
import type { FileInfo, ParsedFile } from '@/types/api'
import { getWorkspace, getFile } from '@/api/endpoints/files'
import { ws } from '@/api/websocket'

export const useCollectionStore = defineStore('collection', () => {
  const files = ref<FileInfo[]>([])
  const openFiles = ref<Map<string, ParsedFile>>(new Map())
  const activeFilePath = ref<string | null>(null)
  const expandedFiles = ref<Set<string>>(new Set())
  const loading = ref(false)
  const error = ref<string | null>(null)

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

  async function loadFiles() {
    loading.value = true
    error.value = null
    try {
      const workspace = await getWorkspace()
      files.value = workspace.files
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to load workspace'
    } finally {
      loading.value = false
    }
  }

  async function openFile(path: string) {
    if (!openFiles.value.has(path)) {
      try {
        const parsed = await getFile(path)
        openFiles.value.set(path, parsed)
      } catch (e) {
        error.value = e instanceof Error ? e.message : `Failed to open ${path}`
        return
      }
    }
    activeFilePath.value = path
    expandedFiles.value.add(path)
  }

  function toggleFileExpanded(path: string) {
    if (expandedFiles.value.has(path)) {
      expandedFiles.value.delete(path)
    } else {
      expandedFiles.value.add(path)
    }
  }

  function closeFile(path: string) {
    openFiles.value.delete(path)
    if (activeFilePath.value === path) {
      const paths = [...openFiles.value.keys()]
      activeFilePath.value = paths.length > 0 ? paths[paths.length - 1] : null
    }
  }

  let initialized = false

  function init() {
    if (initialized) return
    initialized = true
    ws.on('file_changed', () => {
      loadFiles()
      // Refresh all open files, not just the active one
      for (const path of openFiles.value.keys()) {
        getFile(path).then((parsed) => {
          openFiles.value.set(path, parsed)
        }).catch(() => {
          // File may have been deleted; remove from open files
          openFiles.value.delete(path)
          if (activeFilePath.value === path) {
            const remaining = [...openFiles.value.keys()]
            activeFilePath.value = remaining.length > 0 ? remaining[remaining.length - 1] : null
          }
        })
      }
    })
  }

  return { files, openFiles, activeFilePath, expandedFiles, activeFile, fileCount, loading, error, loadFiles, openFile, closeFile, toggleFileExpanded, init }
})
