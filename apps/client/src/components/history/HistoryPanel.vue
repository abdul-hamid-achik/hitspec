<script setup lang="ts">
import { onMounted } from 'vue'
import { Trash2 } from 'lucide-vue-next'
import { useHistoryStore } from '@/stores/history'
import HistoryItem from './HistoryItem.vue'

const historyStore = useHistoryStore()

onMounted(() => {
  historyStore.loadHistory()
})
</script>

<template>
  <div>
    <div class="mb-4 flex items-center justify-between">
      <h3 class="text-sm font-medium text-foreground">Execution History</h3>
      <button
        v-if="historyStore.entries.length > 0"
        class="flex items-center gap-1 rounded-md px-2 py-1 text-xs text-muted-foreground hover:bg-surface-hover hover:text-foreground"
        @click="window.confirm('Clear all history? This cannot be undone.') && historyStore.clearAll()"
      >
        <Trash2 :size="12" /> Clear
      </button>
    </div>
    <div v-if="historyStore.loading" class="text-sm text-muted-foreground">Loading...</div>
    <div v-else-if="historyStore.entries.length === 0" class="text-sm text-muted-foreground">
      No execution history
    </div>
    <div v-else class="space-y-1">
      <HistoryItem
        v-for="entry in historyStore.entries"
        :key="entry.id"
        :entry="entry"
      />
    </div>
  </div>
</template>
