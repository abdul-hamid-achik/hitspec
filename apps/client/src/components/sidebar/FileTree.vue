<script setup lang="ts">
import { ChevronRight, FileText, Folder, FolderOpen, Play, Share2, Trash2 } from 'lucide-vue-next'
import { ref, computed, shallowRef, triggerRef } from 'vue'
import type { FileInfo } from '@/types/api'
import { useCollectionStore } from '@/stores/collection'
import { useRequestStore } from '@/stores/request'
import { useEnvironmentStore } from '@/stores/environment'
import { useConfirm } from '@/composables/useConfirm'
import MethodBadge from '@/components/common/MethodBadge.vue'
import ExportDialog from '@/components/export/ExportDialog.vue'

const { items, depth } = defineProps<{ items: FileInfo[]; depth?: number }>()

const collection = useCollectionStore()
const requestStore = useRequestStore()
const envStore = useEnvironmentStore()
const { confirm } = useConfirm()
const expandedDirs = shallowRef<Set<string>>(new Set())
const exportRequest = ref<import('@/types/api').RequestDTO | null>(null)
const exportFilePath = ref<string | null>(null)
const showExportDialog = ref(false)

// Merge file-level variables with environment variables for export
const exportVariables = computed(() => {
  const vars: Record<string, unknown> = {}
  // File-level variables first (lower priority)
  if (exportFilePath.value) {
    const parsed = collection.openFiles.get(exportFilePath.value)
    if (parsed) {
      for (const v of parsed.variables) {
        vars[v.name] = v.value
      }
    }
  }
  // Environment variables override file-level ones
  const envVars = envStore.activeEnv?.variables ?? {}
  for (const [k, v] of Object.entries(envVars)) {
    vars[k] = v
  }
  return vars
})

function toggleDir(path: string) {
  if (expandedDirs.value.has(path)) {
    expandedDirs.value.delete(path)
  } else {
    expandedDirs.value.add(path)
  }
  triggerRef(expandedDirs)
}

async function handleFileClick(item: FileInfo) {
  // If file is already open and expanded, toggle collapse
  if (collection.activeFilePath === item.path && collection.expandedFiles.has(item.path)) {
    collection.toggleFileExpanded(item.path)
    return
  }
  // If file is open but collapsed, just expand it
  if (collection.openFiles.has(item.path) && !collection.expandedFiles.has(item.path)) {
    collection.toggleFileExpanded(item.path)
    collection.activeFilePath = item.path
    return
  }
  await collection.openFile(item.path)
  const parsed = collection.openFiles.get(item.path)
  if (parsed && parsed.requests.length > 0) {
    requestStore.setActiveRequest(parsed.requests[0], 0)
  }
}

function handleRequestClick(filePath: string, request: import('@/types/api').RequestDTO, index: number) {
  collection.activeFilePath = filePath
  requestStore.setActiveRequest(request, index)
}

async function runFolder(item: FileInfo, event: Event) {
  event.stopPropagation()
  if (requestStore.isExecuting) return

  // Collect all .http/.hitspec file paths under this directory
  const filePaths: string[] = []
  function collectFiles(items: FileInfo[]) {
    for (const f of items) {
      if (f.isDir && f.children) collectFiles(f.children)
      else if (!f.isDir) filePaths.push(f.path)
    }
  }
  if (item.children) collectFiles(item.children)
  if (filePaths.length === 0) return

  // Run each file sequentially and aggregate results
  requestStore.isExecuting = true
  requestStore.error = null
  requestStore.lastResult = null
  try {
    const { executeFile } = await import('@/api/endpoints/execute')
    const allResults: import('@/types/api').RunResult[] = []
    let totalPassed = 0, totalFailed = 0, totalSkipped = 0, totalDuration = 0
    for (const fp of filePaths) {
      const result = await executeFile(fp, envStore.activeEnvName)
      allResults.push(...result.results)
      totalPassed += result.passed
      totalFailed += result.failed
      totalSkipped += result.skipped
      totalDuration += result.duration
    }
    requestStore.lastRunResult = {
      file: item.path,
      duration: totalDuration,
      passed: totalPassed,
      failed: totalFailed,
      skipped: totalSkipped,
      results: allResults,
    }
    if (allResults.length > 0) {
      requestStore.lastResult = allResults[0]
    }
  } catch (e) {
    requestStore.error = e instanceof Error ? e.message : String(e)
  } finally {
    requestStore.isExecuting = false
  }
}

async function runFile(item: FileInfo, event: Event) {
  event.stopPropagation()
  await collection.openFile(item.path)
  requestStore.runFile(item.path, envStore.activeEnvName)
}

async function deleteFile(item: FileInfo, event: Event) {
  event.stopPropagation()
  const ok = await confirm({
    title: 'Delete file',
    message: `Delete ${item.name}? This cannot be undone.`,
    confirmLabel: 'Delete',
    variant: 'destructive',
  })
  if (!ok) return
  await collection.deleteCurrentFile(item.path)
}
</script>

<template>
  <div :role="(depth ?? 0) === 0 ? 'tree' : 'group'">
    <div
      v-for="item in items"
      :key="item.path"
    >
      <div
        role="treeitem"
        tabindex="0"
        class="group flex w-full items-center gap-1 rounded-md px-1.5 py-[5px] text-left text-[13px] transition-colors hover:bg-surface-hover"
        :class="[
          collection.activeFilePath === item.path
            ? 'bg-accent/10 text-foreground'
            : 'text-muted-foreground',
        ]"
        :style="{ paddingLeft: `${(depth ?? 0) * 12 + 6}px` }"
        :aria-expanded="item.isDir ? expandedDirs.has(item.path) : (!item.isDir && item.requestCount && item.requestCount > 0) ? collection.expandedFiles.has(item.path) : undefined"
        @click="item.isDir ? toggleDir(item.path) : handleFileClick(item)"
        @keydown.enter.prevent="item.isDir ? toggleDir(item.path) : handleFileClick(item)"
        @keydown.space.prevent="item.isDir ? toggleDir(item.path) : handleFileClick(item)"
      >
        <ChevronRight
          v-if="item.isDir || (!item.isDir && item.requestCount && item.requestCount > 0)"
          class="h-3 w-3 shrink-0 text-muted-foreground/60 transition-transform"
          :class="{ 'rotate-90': item.isDir ? expandedDirs.has(item.path) : collection.expandedFiles.has(item.path) }"
        />
        <span v-else class="w-3 shrink-0" />
        <FolderOpen v-if="item.isDir && expandedDirs.has(item.path)" class="h-3.5 w-3.5 shrink-0 text-nord-13" />
        <Folder v-else-if="item.isDir" class="h-3.5 w-3.5 shrink-0 text-nord-13/70" />
        <FileText v-else class="h-3.5 w-3.5 shrink-0 text-nord-8/70" />
        <span class="flex-1 truncate">{{ item.name }}</span>
        <span
          v-if="!item.isDir && collection.isFileDirty(item.path)"
          class="h-1.5 w-1.5 shrink-0 rounded-full bg-nord-13"
          title="Unsaved changes"
        />
        <button
          v-if="item.isDir"
          aria-label="Run all files in folder"
          class="invisible rounded p-0.5 text-muted-foreground/60 transition-colors hover:bg-accent/20 hover:text-accent group-hover:visible"
          title="Run all files in folder"
          @click.stop="runFolder(item, $event)"
        >
          <Play class="h-3 w-3" />
        </button>
        <button
          v-else-if="item.requestCount && item.requestCount > 0"
          aria-label="Run file"
          class="invisible rounded p-0.5 text-muted-foreground/60 transition-colors hover:bg-accent/20 hover:text-accent group-hover:visible"
          title="Run file"
          @click.stop="runFile(item, $event)"
        >
          <Play class="h-3 w-3" />
        </button>
        <button
          v-if="!item.isDir"
          aria-label="Delete file"
          class="invisible rounded p-0.5 text-muted-foreground/60 transition-colors hover:bg-destructive/20 hover:text-destructive group-hover:visible"
          title="Delete file"
          @click.stop="deleteFile(item, $event)"
        >
          <Trash2 class="h-3 w-3" />
        </button>
        <span
          v-if="!item.isDir && item.requestCount && item.requestCount > 0"
          class="rounded-full px-1.5 text-[10px] tabular-nums text-muted-foreground/60 group-hover:text-muted-foreground/70"
        >
          {{ item.requestCount }}
        </span>
      </div>

      <!-- Directory children -->
      <FileTree
        v-if="item.isDir && item.children && expandedDirs.has(item.path)"
        :items="item.children"
        :depth="(depth ?? 0) + 1"
      />

      <!-- Request sub-list for expanded files -->
      <div
        v-if="!item.isDir && collection.expandedFiles.has(item.path) && collection.openFiles.get(item.path)"
      >
        <div
          v-for="(req, idx) in collection.openFiles.get(item.path)!.requests"
          :key="idx"
          class="group flex items-center"
        >
          <button
            class="flex flex-1 items-center gap-1.5 rounded-md px-1.5 py-[3px] text-left text-[12px] transition-colors hover:bg-surface-hover"
            :class="[
              requestStore.activeRequest === req ? 'bg-accent/10 text-foreground' : 'text-muted-foreground',
            ]"
            :style="{ paddingLeft: `${(depth ?? 0) * 12 + 24}px` }"
            @click="handleRequestClick(item.path, req, idx)"
          >
            <MethodBadge :method="req.method" size="sm" />
            <span class="flex-1 truncate">{{ req.name || req.url }}</span>
          </button>
          <button
            aria-label="Export request"
            class="invisible mr-1 rounded p-0.5 text-muted-foreground/40 transition-colors hover:bg-accent/20 hover:text-accent group-hover:visible"
            title="Export request"
            @click.stop="exportRequest = req; exportFilePath = item.path; showExportDialog = true"
          >
            <Share2 class="h-3 w-3" />
          </button>
        </div>
      </div>
    </div>

    <ExportDialog
      v-if="exportRequest"
      v-model="showExportDialog"
      :request="exportRequest"
      :variables="exportVariables"
    />
  </div>
</template>
