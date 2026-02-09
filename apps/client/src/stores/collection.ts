import { ref, computed } from 'vue'
import { defineStore } from 'pinia'
import type { FileInfo, ParsedFile } from '@/types/api'
import { getWorkspace, getFile } from '@/api/endpoints/files'
import { ws } from '@/api/websocket'

export const useCollectionStore = defineStore('collection', () => {
  const files = ref<FileInfo[]>([])
  const openFiles = ref<Map<string, ParsedFile>>(new Map())
  const activeFilePath = ref<string | null>(null)
  const loading = ref(false)

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
    try {
      const workspace = await getWorkspace()
      files.value = workspace.files
    } finally {
      loading.value = false
    }
  }

  async function openFile(path: string) {
    if (!openFiles.value.has(path)) {
      const parsed = await getFile(path)
      openFiles.value.set(path, parsed)
    }
    activeFilePath.value = path
  }

  function closeFile(path: string) {
    openFiles.value.delete(path)
    if (activeFilePath.value === path) {
      const paths = [...openFiles.value.keys()]
      activeFilePath.value = paths.length > 0 ? paths[paths.length - 1] : null
    }
  }

  function init() {
    ws.on('file_changed', () => {
      loadFiles()
      if (activeFilePath.value && openFiles.value.has(activeFilePath.value)) {
        getFile(activeFilePath.value).then((parsed) => {
          openFiles.value.set(activeFilePath.value!, parsed)
        })
      }
    })
  }

  return { files, openFiles, activeFilePath, activeFile, fileCount, loading, loadFiles, openFile, closeFile, init }
})
