<script setup lang="ts">
import AppShell from '@/components/layout/AppShell.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import MethodBadge from '@/components/common/MethodBadge.vue'
import StatusBadge from '@/components/common/StatusBadge.vue'
import { History, Trash2, CheckCircle, XCircle, Clock } from 'lucide-vue-next'
import { useHistoryStore } from '@/stores/history'
import { useCollectionStore } from '@/stores/collection'
import { useRequestStore } from '@/stores/request'
import { onMounted } from 'vue'
import { useRouter } from 'vue-router'
import dayjs from 'dayjs'
import type { HistoryEntry } from '@/types/api'

const historyStore = useHistoryStore()
const collection = useCollectionStore()
const requestStore = useRequestStore()
const router = useRouter()

onMounted(() => historyStore.loadHistory())

async function openHistoryEntry(entry: HistoryEntry) {
  await collection.openFile(entry.file)
  const parsed = collection.openFiles.get(entry.file)
  if (parsed) {
    const req = parsed.requests.find((r) => r.name === entry.requestName)
    if (req) {
      requestStore.setActiveRequest(req, parsed.requests.indexOf(req))
    }
  }
  router.push('/')
}

function confirmClear() {
  if (window.confirm('Clear all history? This cannot be undone.')) {
    historyStore.clearAll()
  }
}
</script>

<template>
  <AppShell>
    <div class="h-full overflow-auto p-6">
      <div class="mb-4 flex items-center justify-between">
        <h1 class="text-lg font-semibold text-foreground">History</h1>
        <button
          v-if="historyStore.entries.length > 0"
          class="flex items-center gap-1.5 rounded-md border border-border px-2.5 py-1 text-xs text-muted-foreground transition-colors hover:border-destructive/50 hover:text-destructive"
          @click="confirmClear"
        >
          <Trash2 class="h-3 w-3" />
          Clear All
        </button>
      </div>

      <LoadingSpinner v-if="historyStore.loading" label="Loading history..." />
      <div v-else-if="historyStore.entries.length > 0" class="space-y-1.5">
        <button
          v-for="entry in historyStore.entries"
          :key="entry.id"
          class="flex w-full items-center gap-3 rounded-lg border border-border bg-surface p-3 text-left transition-colors hover:bg-surface-hover"
          @click="openHistoryEntry(entry)"
        >
          <component
            :is="entry.passed ? CheckCircle : XCircle"
            class="h-4 w-4 shrink-0"
            :class="entry.passed ? 'text-success/60' : 'text-destructive/60'"
          />
          <MethodBadge :method="entry.method" size="sm" />
          <span class="flex-1 truncate font-mono text-xs text-foreground/80">{{ entry.url }}</span>
          <StatusBadge :code="entry.statusCode" />
          <span class="flex items-center gap-1 text-xs tabular-nums text-muted-foreground/50">
            <Clock class="h-3 w-3" />
            {{ entry.duration }}ms
          </span>
          <span class="text-[11px] text-muted-foreground/40">{{ dayjs(entry.timestamp).format('HH:mm:ss') }}</span>
        </button>
      </div>
      <EmptyState v-else :icon="History" title="No history yet" description="Execute requests to see them here" />
    </div>
  </AppShell>
</template>
