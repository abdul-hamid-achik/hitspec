<script setup lang="ts">
import { ref } from 'vue'
import { ChevronRight, FileText, Folder } from 'lucide-vue-next'
import type { FileInfo } from '@/types/api'
import { useCollectionStore } from '@/stores/collection'
import { cn } from '@/lib/utils'

const { file, depth } = defineProps<{
  file: FileInfo
  depth: number
}>()

const collection = useCollectionStore()
const expanded = ref(file.isDir)

function toggle() {
  if (file.isDir) {
    expanded.value = !expanded.value
  } else {
    collection.openFile(file.path)
  }
}

const isActive = () => collection.activeFilePath === file.path
</script>

<template>
  <div>
    <button
      :class="cn(
        'flex w-full items-center gap-1.5 rounded-sm px-1.5 py-1 text-left text-sm text-muted-foreground transition-colors hover:bg-surface-hover hover:text-foreground',
        isActive() && 'bg-surface text-foreground'
      )"
      :style="{ paddingLeft: depth * 12 + 6 + 'px' }"
      @click="toggle"
    >
      <ChevronRight
        v-if="file.isDir"
        :size="14"
        class="shrink-0 transition-transform"
        :class="{ 'rotate-90': expanded }"
      />
      <component :is="file.isDir ? Folder : FileText" :size="14" class="shrink-0 text-nord-9" />
      <span class="truncate">{{ file.name }}</span>
      <span v-if="!file.isDir && (file.requestCount ?? 0) > 0" class="ml-auto text-xs text-nord-3">
        {{ file.requestCount }}
      </span>
    </button>
    <div v-if="file.isDir && expanded && file.children">
      <FileTreeItem
        v-for="child in file.children"
        :key="child.path"
        :file="child"
        :depth="depth + 1"
      />
    </div>
  </div>
</template>
