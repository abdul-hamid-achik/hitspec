<script setup lang="ts">
import dayjs from 'dayjs'
import type { HistoryEntry } from '@/types/api'
import MethodBadge from '@/components/common/MethodBadge.vue'
import StatusBadge from '@/components/response/StatusBadge.vue'
import { CheckCircle2, XCircle } from 'lucide-vue-next'

defineProps<{ entry: HistoryEntry }>()
</script>

<template>
  <div class="flex items-center gap-3 rounded-md border border-border bg-nord-0 px-3 py-2">
    <component
      :is="entry.passed ? CheckCircle2 : XCircle"
      :size="14"
      :class="entry.passed ? 'text-success' : 'text-destructive'"
    />
    <MethodBadge :method="entry.method" />
    <div class="min-w-0 flex-1">
      <div class="truncate font-mono text-sm text-foreground">{{ entry.url }}</div>
      <div class="text-xs text-muted-foreground">
        {{ entry.requestName }} - {{ entry.filePath }}
      </div>
    </div>
    <StatusBadge :code="entry.statusCode" />
    <span class="text-xs text-muted-foreground">{{ entry.duration }}ms</span>
    <span class="text-xs text-nord-3">{{ dayjs(entry.timestamp).format('HH:mm:ss') }}</span>
  </div>
</template>
