<script setup lang="ts">
import { ChevronRight, FileText, Folder } from 'lucide-vue-next'
import { ref } from 'vue'
import type { FileInfo } from '@/types/api'
import { useCollectionStore } from '@/stores/collection'
import MethodBadge from '@/components/common/MethodBadge.vue'

defineProps<{ items: FileInfo[]; depth?: number }>()

const collection = useCollectionStore()
const expanded = ref<Set<string>>(new Set())

function toggle(path: string) {
  if (expanded.value.has(path)) {
    expanded.value.delete(path)
  } else {
    expanded.value.add(path)
  }
}
</script>

<template>
  <div>
    <div
      v-for="item in items"
      :key="item.path"
    >
      <button
        class="flex w-full items-center gap-1.5 rounded px-1 py-1 text-left text-sm transition-colors hover:bg-surface-hover"
        :class="[
          collection.activeFilePath === item.path ? 'bg-surface-hover text-foreground' : 'text-muted-foreground',
        ]"
        :style="{ paddingLeft: `${(depth ?? 0) * 12 + 4}px` }"
        @click="item.isDir ? toggle(item.path) : collection.openFile(item.path)"
      >
        <ChevronRight
          v-if="item.isDir"
          class="h-3.5 w-3.5 shrink-0 transition-transform"
          :class="{ 'rotate-90': expanded.has(item.path) }"
        />
        <Folder v-if="item.isDir" class="h-3.5 w-3.5 shrink-0 text-nord-13" />
        <FileText v-else class="h-3.5 w-3.5 shrink-0 text-nord-8" />
        <span class="flex-1 truncate">{{ item.name }}</span>
        <span v-if="!item.isDir && item.requestCount > 0" class="text-xs text-muted-foreground/60">
          {{ item.requestCount }}
        </span>
      </button>
      <FileTree
        v-if="item.isDir && item.children && expanded.has(item.path)"
        :items="item.children"
        :depth="(depth ?? 0) + 1"
      />
    </div>
  </div>
</template>
