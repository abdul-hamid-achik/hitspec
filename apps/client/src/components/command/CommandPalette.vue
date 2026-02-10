<script setup lang="ts">
import { ref, computed, watch, nextTick } from 'vue'
import { DialogRoot, DialogPortal, DialogOverlay, DialogContent, DialogTitle } from 'reka-ui'
import {
  Search, FileText, Zap, Server, History, Settings, Import, Play,
  Keyboard, ArrowRight, Command, FileCheck, Video,
} from 'lucide-vue-next'
import { useRouter } from 'vue-router'
import { useCollectionStore } from '@/stores/collection'
import { useRequestStore } from '@/stores/request'
import MethodBadge from '@/components/common/MethodBadge.vue'
import type { FileInfo, RequestDTO } from '@/types/api'

const props = defineProps<{ open: boolean }>()
const emit = defineEmits<{
  'update:open': [value: boolean]
  'show-shortcuts': []
}>()

const router = useRouter()
const collection = useCollectionStore()
const requestStore = useRequestStore()

const query = ref('')
const selectedIndex = ref(0)
const inputRef = ref<HTMLInputElement | null>(null)

interface CommandItem {
  id: string
  label: string
  description?: string
  icon: typeof Search
  group: string
  action: () => void
  method?: string
}

const navCommands: CommandItem[] = [
  { id: 'nav-workspace', label: 'Workspace', description: 'View and edit requests', icon: FileText, group: 'Navigate', action: () => router.push('/') },
  { id: 'nav-stress', label: 'Stress Testing', description: 'Load test your APIs', icon: Zap, group: 'Navigate', action: () => router.push('/stress') },
  { id: 'nav-mock', label: 'Mock Server', description: 'Mock API endpoints', icon: Server, group: 'Navigate', action: () => router.push('/mock') },
  { id: 'nav-contract', label: 'Contract Testing', description: 'Verify API contracts', icon: FileCheck, group: 'Navigate', action: () => router.push('/contract') },
  { id: 'nav-record', label: 'Record Proxy', description: 'Record HTTP traffic', icon: Video, group: 'Navigate', action: () => router.push('/record') },
  { id: 'nav-history', label: 'History', description: 'View past executions', icon: History, group: 'Navigate', action: () => router.push('/history') },
  { id: 'nav-import', label: 'Import', description: 'Import from cURL, Insomnia', icon: Import, group: 'Navigate', action: () => router.push('/import') },
  { id: 'nav-settings', label: 'Settings', description: 'Configure hitspec', icon: Settings, group: 'Navigate', action: () => router.push('/settings') },
]

const actionCommands: CommandItem[] = [
  {
    id: 'action-run',
    label: 'Run Current File',
    description: 'Execute all requests in active file',
    icon: Play,
    group: 'Actions',
    action: () => {
      if (collection.activeFilePath) {
        requestStore.runFile(collection.activeFilePath)
      }
    },
  },
  {
    id: 'action-shortcuts',
    label: 'Keyboard Shortcuts',
    description: 'View all keyboard shortcuts',
    icon: Keyboard,
    group: 'Actions',
    action: () => emit('show-shortcuts'),
  },
]

function flattenFiles(items: FileInfo[]): FileInfo[] {
  const result: FileInfo[] = []
  for (const item of items) {
    if (!item.isDir) result.push(item)
    if (item.children) result.push(...flattenFiles(item.children))
  }
  return result
}

const fileCommands = computed<CommandItem[]>(() => {
  const files = flattenFiles(collection.files)
  return files.map((f) => ({
    id: `file-${f.path}`,
    label: f.name,
    description: f.path,
    icon: FileText,
    group: 'Files',
    action: () => collection.openFile(f.path),
  }))
})

const requestCommands = computed<CommandItem[]>(() => {
  const cmds: CommandItem[] = []
  for (const [path, parsed] of collection.openFiles) {
    for (const req of parsed.requests) {
      cmds.push({
        id: `req-${path}-${req.name}`,
        label: req.name || req.url,
        description: `${req.method} ${req.url}`,
        icon: ArrowRight,
        group: 'Requests',
        method: req.method,
        action: () => {
          collection.activeFilePath = path
          requestStore.setActiveRequest(req, parsed.requests.indexOf(req))
        },
      })
    }
  }
  return cmds
})

const filtered = computed(() => {
  const q = query.value.toLowerCase().trim()
  const all = [...actionCommands, ...navCommands, ...fileCommands.value, ...requestCommands.value]
  if (!q) return all.slice(0, 20)
  return all.filter(
    (item) =>
      item.label.toLowerCase().includes(q) ||
      item.description?.toLowerCase().includes(q) ||
      item.group.toLowerCase().includes(q),
  ).slice(0, 20)
})

const groups = computed(() => {
  const map = new Map<string, CommandItem[]>()
  for (const item of filtered.value) {
    const list = map.get(item.group) || []
    list.push(item)
    map.set(item.group, list)
  }
  return map
})

watch(() => query.value, () => {
  selectedIndex.value = 0
})

watch(() => props.open, (isOpen) => {
  if (isOpen) {
    query.value = ''
    selectedIndex.value = 0
    nextTick(() => inputRef.value?.focus())
  }
})

function close() {
  emit('update:open', false)
}

function execute(item: CommandItem) {
  close()
  item.action()
}

function onKeydown(e: KeyboardEvent) {
  const items = filtered.value
  if (items.length === 0) return
  if (e.key === 'ArrowDown') {
    e.preventDefault()
    selectedIndex.value = (selectedIndex.value + 1) % items.length
  } else if (e.key === 'ArrowUp') {
    e.preventDefault()
    selectedIndex.value = (selectedIndex.value - 1 + items.length) % items.length
  } else if (e.key === 'Enter' && items[selectedIndex.value]) {
    e.preventDefault()
    execute(items[selectedIndex.value])
  }
}
</script>

<template>
  <DialogRoot :open="open" @update:open="emit('update:open', $event)">
    <DialogPortal>
      <DialogOverlay class="fixed inset-0 z-50 bg-black/60 backdrop-blur-sm data-[state=open]:animate-in data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0" />
      <DialogContent
        class="fixed left-1/2 top-[20%] z-50 w-full max-w-lg -translate-x-1/2 rounded-xl border border-border bg-surface shadow-2xl data-[state=open]:animate-in data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0 data-[state=closed]:zoom-out-95 data-[state=open]:zoom-in-95 data-[state=closed]:slide-out-to-left-1/2 data-[state=open]:slide-in-from-left-1/2"
        @keydown="onKeydown"
      >
        <DialogTitle class="sr-only">Command Palette</DialogTitle>
        <!-- Search input -->
        <div class="flex items-center gap-2 border-b border-border px-4 py-3">
          <Search class="h-4 w-4 shrink-0 text-muted-foreground" />
          <input
            ref="inputRef"
            v-model="query"
            placeholder="Type a command or search..."
            class="flex-1 bg-transparent text-sm text-foreground placeholder:text-muted-foreground/50 focus:outline-none"
          />
          <kbd class="rounded border border-border bg-background px-1.5 py-0.5 text-[10px] font-medium text-muted-foreground">ESC</kbd>
        </div>

        <!-- Results -->
        <div class="max-h-72 overflow-y-auto p-1.5">
          <div v-if="filtered.length === 0" class="px-3 py-6 text-center text-sm text-muted-foreground">
            No results found
          </div>
          <template v-for="[group, items] in groups" :key="group">
            <div class="px-2.5 pt-2 pb-1 text-[10px] font-semibold uppercase tracking-wider text-muted-foreground/60">
              {{ group }}
            </div>
            <button
              v-for="(item, i) in items"
              :key="item.id"
              class="flex w-full items-center gap-2.5 rounded-lg px-2.5 py-2 text-left text-sm transition-colors"
              :class="filtered.indexOf(item) === selectedIndex ? 'bg-accent/15 text-foreground' : 'text-muted-foreground hover:bg-surface-hover hover:text-foreground'"
              @click="execute(item)"
              @mouseenter="selectedIndex = filtered.indexOf(item)"
            >
              <MethodBadge v-if="item.method" :method="item.method" size="sm" />
              <component v-else :is="item.icon" class="h-4 w-4 shrink-0 text-muted-foreground/60" />
              <div class="min-w-0 flex-1">
                <div class="truncate">{{ item.label }}</div>
                <div v-if="item.description" class="truncate text-xs text-muted-foreground/50">{{ item.description }}</div>
              </div>
              <ArrowRight v-if="filtered.indexOf(item) === selectedIndex" class="h-3.5 w-3.5 shrink-0 text-muted-foreground/40" />
            </button>
          </template>
        </div>

        <!-- Footer hint -->
        <div class="flex items-center gap-3 border-t border-border px-4 py-2 text-[10px] text-muted-foreground/50">
          <span class="flex items-center gap-1"><kbd class="rounded border border-border/50 px-1 py-px">&#8593;&#8595;</kbd> navigate</span>
          <span class="flex items-center gap-1"><kbd class="rounded border border-border/50 px-1 py-px">&#9166;</kbd> select</span>
          <span class="flex items-center gap-1"><kbd class="rounded border border-border/50 px-1 py-px">esc</kbd> close</span>
        </div>
      </DialogContent>
    </DialogPortal>
  </DialogRoot>
</template>
