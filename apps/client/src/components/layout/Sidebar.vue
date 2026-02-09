<script setup lang="ts">
import { FolderTree, Zap, Server, History, Settings, Import } from 'lucide-vue-next'
import FileTree from '@/components/sidebar/FileTree.vue'
import { useCollectionStore } from '@/stores/collection'

defineProps<{ width: number }>()

const collection = useCollectionStore()
</script>

<template>
  <aside
    class="flex flex-col border-r border-border bg-surface"
    :style="{ width: `${width}px`, minWidth: `${width}px` }"
  >
    <div class="flex items-center justify-between border-b border-border px-3 py-2">
      <span class="text-xs font-semibold uppercase tracking-wider text-muted-foreground">Files</span>
      <span class="text-xs text-muted-foreground">{{ collection.fileCount }}</span>
    </div>
    <div class="flex-1 overflow-auto p-2">
      <FileTree :items="collection.files" />
    </div>
    <nav class="border-t border-border p-2">
      <router-link
        v-for="item in [
          { to: '/', icon: FolderTree, label: 'Workspace' },
          { to: '/stress', icon: Zap, label: 'Stress Test' },
          { to: '/mock', icon: Server, label: 'Mock Server' },
          { to: '/history', icon: History, label: 'History' },
          { to: '/import', icon: Import, label: 'Import' },
          { to: '/settings', icon: Settings, label: 'Settings' },
        ]"
        :key="item.to"
        :to="item.to"
        class="flex items-center gap-2 rounded px-2 py-1.5 text-sm text-muted-foreground transition-colors hover:bg-surface-hover hover:text-foreground"
        active-class="bg-surface-hover text-foreground"
      >
        <component :is="item.icon" class="h-4 w-4" />
        {{ item.label }}
      </router-link>
    </nav>
  </aside>
</template>
