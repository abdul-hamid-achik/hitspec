<script setup lang="ts">
import { ref, computed } from 'vue'
import {
  FolderTree, Zap, Server, History, Settings, Import, Cookie,
  ChevronDown, Search, PanelLeftClose, FileCheck, Video, AlertCircle,
} from 'lucide-vue-next'
import FileTree from '@/components/sidebar/FileTree.vue'
import SearchInput from '@/components/sidebar/SearchInput.vue'
import { useCollectionStore } from '@/stores/collection'

const { width, collapsed } = defineProps<{ width: number; collapsed?: boolean }>()
const emit = defineEmits<{ collapse: [] }>()

const collection = useCollectionStore()
const showSearch = ref(false)
const searchQuery = ref('')
const filesExpanded = ref(true)

const filteredFiles = computed(() => {
  if (!searchQuery.value) return collection.files
  const q = searchQuery.value.toLowerCase()
  function filterTree(items: import('@/types/api').FileInfo[]): import('@/types/api').FileInfo[] {
    return items.filter((item) => {
      if (item.name.toLowerCase().includes(q)) return true
      if (item.children) {
        const filtered = filterTree(item.children)
        if (filtered.length > 0) return true
      }
      return false
    }).map((item) => {
      if (!item.children) return item
      return { ...item, children: filterTree(item.children) }
    })
  }
  return filterTree(collection.files)
})

const navItems = [
  { to: '/', icon: FolderTree, label: 'Workspace' },
  { to: '/stress', icon: Zap, label: 'Stress Test' },
  { to: '/mock', icon: Server, label: 'Mock Server' },
  { to: '/contract', icon: FileCheck, label: 'Contracts' },
  { to: '/record', icon: Video, label: 'Record' },
  { to: '/history', icon: History, label: 'History' },
  { to: '/import', icon: Import, label: 'Import' },
  { to: '/cookies', icon: Cookie, label: 'Cookies' },
  { to: '/settings', icon: Settings, label: 'Settings' },
]

function handleSearch(value: string) {
  searchQuery.value = value
}
</script>

<template>
  <aside
    v-if="!collapsed"
    class="flex flex-col border-r border-border bg-surface"
    :style="{ width: `${width}px`, minWidth: `${width}px` }"
  >
    <!-- Files section header -->
    <div class="flex items-center justify-between border-b border-border px-3 py-2">
      <button
        class="flex items-center gap-1 text-xs font-semibold uppercase tracking-wider text-muted-foreground transition-colors hover:text-foreground"
        @click="filesExpanded = !filesExpanded"
      >
        <ChevronDown
          class="h-3 w-3 transition-transform"
          :class="{ '-rotate-90': !filesExpanded }"
        />
        Files
      </button>
      <div class="flex items-center gap-1">
        <span class="text-[10px] tabular-nums text-muted-foreground/60">{{ collection.fileCount }}</span>
        <button
          aria-label="Search files"
          class="rounded p-0.5 text-muted-foreground/60 transition-colors hover:bg-surface-hover hover:text-foreground"
          title="Search files"
          @click="showSearch = !showSearch"
        >
          <Search class="h-3.5 w-3.5" />
        </button>
        <button
          aria-label="Collapse sidebar"
          class="rounded p-0.5 text-muted-foreground/60 transition-colors hover:bg-surface-hover hover:text-foreground"
          title="Collapse sidebar"
          @click="emit('collapse')"
        >
          <PanelLeftClose class="h-3.5 w-3.5" />
        </button>
      </div>
    </div>

    <!-- Search input (collapsible) -->
    <div v-if="showSearch" class="border-b border-border px-2 py-1.5">
      <SearchInput @search="handleSearch" />
    </div>

    <!-- File tree -->
    <div v-if="filesExpanded" class="min-h-0 flex-1 overflow-auto p-1.5">
      <div v-if="collection.loading" class="flex items-center justify-center py-6">
        <div class="h-4 w-4 animate-spin rounded-full border-2 border-accent/30 border-t-accent" />
      </div>
      <div v-else-if="collection.error && collection.files.length === 0" class="flex flex-col items-center gap-2 px-2 py-4 text-center">
        <AlertCircle class="h-5 w-5 text-destructive/60" />
        <p class="text-xs text-destructive/80">{{ collection.error }}</p>
        <button
          class="rounded-md border border-border px-2 py-0.5 text-[11px] text-muted-foreground transition-colors hover:bg-surface-hover hover:text-foreground"
          @click="collection.loadFiles()"
        >
          Retry
        </button>
      </div>
      <div v-else-if="filteredFiles.length === 0" class="px-2 py-4 text-center text-xs text-muted-foreground/60">
        {{ searchQuery ? 'No files match your search' : 'No files found' }}
      </div>
      <FileTree v-else :items="filteredFiles" />
    </div>
    <div v-else class="flex-1" />

    <!-- Bottom nav -->
    <nav aria-label="Main navigation" class="shrink-0 border-t border-border p-1.5 overflow-y-auto max-h-[50%]">
      <router-link
        v-for="item in navItems"
        :key="item.to"
        :to="item.to"
        class="flex items-center gap-2 rounded-md px-2.5 py-1.5 text-sm text-muted-foreground transition-colors hover:bg-surface-hover hover:text-foreground"
        active-class="bg-accent/10 text-foreground"
      >
        <component :is="item.icon" class="h-4 w-4 shrink-0" />
        {{ item.label }}
      </router-link>
    </nav>
  </aside>
</template>
