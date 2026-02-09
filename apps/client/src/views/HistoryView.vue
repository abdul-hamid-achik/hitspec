<script setup lang="ts">
import AppShell from '@/components/layout/AppShell.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import MethodBadge from '@/components/common/MethodBadge.vue'
import StatusBadge from '@/components/common/StatusBadge.vue'
import { History, Trash2 } from 'lucide-vue-next'
import { useHistoryStore } from '@/stores/history'
import { onMounted } from 'vue'
import dayjs from 'dayjs'

const historyStore = useHistoryStore()
onMounted(() => historyStore.loadHistory())
</script>

<template>
  <AppShell>
    <div class="p-6">
      <div class="mb-4 flex items-center justify-between">
        <h1 class="text-lg font-semibold text-foreground">History</h1>
        <button
          v-if="historyStore.entries.length > 0"
          class="flex items-center gap-1 text-xs text-muted-foreground hover:text-destructive"
          @click="historyStore.clearAll()"
        >
          <Trash2 class="h-3.5 w-3.5" />
          Clear
        </button>
      </div>
      <div v-if="historyStore.entries.length > 0" class="space-y-2">
        <div
          v-for="entry in historyStore.entries"
          :key="entry.id"
          class="flex items-center gap-3 rounded border border-border bg-surface p-3"
        >
          <MethodBadge :method="entry.method" size="sm" />
          <span class="flex-1 truncate font-mono text-sm text-foreground">{{ entry.url }}</span>
          <StatusBadge :code="entry.statusCode" />
          <span class="text-xs text-muted-foreground">{{ entry.duration }}ms</span>
          <span class="text-xs text-muted-foreground">{{ dayjs(entry.timestamp).format('HH:mm:ss') }}</span>
        </div>
      </div>
      <EmptyState v-else :icon="History" title="No history" description="Execute requests to see them here" />
    </div>
  </AppShell>
</template>
