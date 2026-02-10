<script setup lang="ts">
import { ChevronRight, FileText, Folder, FolderOpen } from 'lucide-vue-next'
import { ref } from 'vue'
import type { FileInfo } from '@/types/api'
import { useCollectionStore } from '@/stores/collection'
import { useRequestStore } from '@/stores/request'
import MethodBadge from '@/components/common/MethodBadge.vue'

defineProps<{ items: FileInfo[]; depth?: number }>()

const collection = useCollectionStore()
const requestStore = useRequestStore()
const expandedDirs = ref<Set<string>>(new Set())

function toggleDir(path: string) {
  if (expandedDirs.value.has(path)) {
    expandedDirs.value.delete(path)
  } else {
    expandedDirs.value.add(path)
  }
}

async function handleFileClick(item: FileInfo) {
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
</script>

<template>
  <div>
    <div
      v-for="item in items"
      :key="item.path"
    >
      <button
        class="group flex w-full items-center gap-1 rounded-md px-1.5 py-[5px] text-left text-[13px] transition-colors hover:bg-surface-hover"
        :class="[
          collection.activeFilePath === item.path
            ? 'bg-accent/10 text-foreground'
            : 'text-muted-foreground',
        ]"
        :style="{ paddingLeft: `${(depth ?? 0) * 12 + 6}px` }"
        @click="item.isDir ? toggleDir(item.path) : handleFileClick(item)"
      >
        <ChevronRight
          v-if="item.isDir || (!item.isDir && item.requestCount && item.requestCount > 0)"
          class="h-3 w-3 shrink-0 text-muted-foreground/40 transition-transform"
          :class="{ 'rotate-90': item.isDir ? expandedDirs.has(item.path) : collection.expandedFiles.has(item.path) }"
        />
        <span v-else class="w-3 shrink-0" />
        <FolderOpen v-if="item.isDir && expandedDirs.has(item.path)" class="h-3.5 w-3.5 shrink-0 text-nord-13" />
        <Folder v-else-if="item.isDir" class="h-3.5 w-3.5 shrink-0 text-nord-13/70" />
        <FileText v-else class="h-3.5 w-3.5 shrink-0 text-nord-8/70" />
        <span class="flex-1 truncate">{{ item.name }}</span>
        <span
          v-if="!item.isDir && item.requestCount && item.requestCount > 0"
          class="rounded-full px-1.5 text-[10px] tabular-nums text-muted-foreground/40 group-hover:text-muted-foreground/60"
        >
          {{ item.requestCount }}
        </span>
      </button>

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
        <button
          v-for="(req, idx) in collection.openFiles.get(item.path)!.requests"
          :key="idx"
          class="flex w-full items-center gap-1.5 rounded-md px-1.5 py-[3px] text-left text-[12px] transition-colors hover:bg-surface-hover"
          :class="[
            requestStore.activeRequest === req ? 'bg-accent/10 text-foreground' : 'text-muted-foreground',
          ]"
          :style="{ paddingLeft: `${(depth ?? 0) * 12 + 24}px` }"
          @click="handleRequestClick(item.path, req, idx)"
        >
          <MethodBadge :method="req.method" size="sm" />
          <span class="flex-1 truncate">{{ req.name || req.url }}</span>
        </button>
      </div>
    </div>
  </div>
</template>
