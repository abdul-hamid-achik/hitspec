<script setup lang="ts">
import { computed, ref } from 'vue'
import type { ExecuteResult, RunResult } from '@/types/api'
import { useSelection } from '@/composables/useSelection'
import StatusBadge from '@/components/common/StatusBadge.vue'
import MethodBadge from '@/components/common/MethodBadge.vue'
import { CheckCircle, XCircle, MinusCircle, Clock, Zap, Square, CheckSquare, MinusSquare } from 'lucide-vue-next'

const { results } = defineProps<{ results: ExecuteResult }>()
const emit = defineEmits<{ select: [result: RunResult] }>()

interface IndexedResult {
  id: number
  result: RunResult
}

const sortedResults = computed<IndexedResult[]>(() => {
  return [...results.results]
    .map((r, i) => ({ id: i, result: r }))
    .sort((a, b) => {
      const order = (r: RunResult) => r.skipped ? 2 : r.passed ? 1 : 0
      return order(a.result) - order(b.result)
    })
})

const { selectedItems, allSelected, someSelected, isSelected, toggle, toggleAll, deselectAll } = useSelection(sortedResults)

function resultIcon(result: RunResult) {
  if (result.skipped) return MinusCircle
  return result.passed ? CheckCircle : XCircle
}

function resultIconClass(result: RunResult) {
  if (result.skipped) return 'text-muted-foreground'
  return result.passed ? 'text-success' : 'text-destructive'
}
</script>

<template>
  <div class="flex h-full flex-col">
    <!-- Summary bar -->
    <div class="flex items-center gap-3 border-b border-border px-4 py-2.5">
      <button
        class="shrink-0 text-muted-foreground/60 hover:text-foreground"
        aria-label="Toggle select all"
        @click="toggleAll()"
      >
        <CheckSquare v-if="allSelected" class="h-3.5 w-3.5" />
        <MinusSquare v-else-if="someSelected" class="h-3.5 w-3.5" />
        <Square v-else class="h-3.5 w-3.5" />
      </button>
      <div class="flex items-center gap-2 text-xs font-medium">
        <span class="text-success">{{ results.passed }} passed</span>
        <span v-if="results.failed > 0" class="text-destructive">{{ results.failed }} failed</span>
        <span v-if="results.skipped > 0" class="text-muted-foreground">{{ results.skipped }} skipped</span>
      </div>
      <div class="flex-1" />
      <template v-if="selectedItems.length > 0">
        <span class="text-[11px] text-accent">{{ selectedItems.length }} selected</span>
        <button
          class="text-[11px] text-muted-foreground hover:text-foreground"
          @click="deselectAll()"
        >
          Deselect
        </button>
      </template>
      <div class="flex items-center gap-1 text-xs text-muted-foreground">
        <Clock class="h-3.5 w-3.5" />
        <span>{{ results.duration }}ms</span>
      </div>
      <div class="flex items-center gap-1 text-xs text-muted-foreground">
        <Zap class="h-3.5 w-3.5" />
        <span>{{ results.results.length }} requests</span>
      </div>
    </div>

    <!-- Results list -->
    <div class="flex-1 overflow-auto">
      <div
        v-for="item in sortedResults"
        :key="item.id"
        class="flex w-full items-center gap-2.5 border-b border-border px-4 py-2 text-left transition-colors hover:bg-surface-hover"
        :class="{ 'bg-accent/5': isSelected(item) }"
      >
        <button
          class="shrink-0 text-muted-foreground/60 hover:text-foreground"
          aria-label="Toggle selection"
          @click.stop="toggle(item, $event)"
        >
          <CheckSquare v-if="isSelected(item)" class="h-3.5 w-3.5 text-accent" />
          <Square v-else class="h-3.5 w-3.5" />
        </button>
        <button
          class="flex flex-1 items-center gap-2.5"
          @click="emit('select', item.result)"
        >
          <component
            :is="resultIcon(item.result)"
            class="h-4 w-4 shrink-0"
            :class="resultIconClass(item.result)"
          />
          <MethodBadge v-if="item.result.request?.method" :method="item.result.request.method" size="sm" />
          <div class="flex flex-1 flex-col gap-0 truncate">
            <span class="truncate text-xs text-foreground">{{ item.result.name }}</span>
            <span v-if="item.result.description" class="truncate text-[10px] text-muted-foreground">{{ item.result.description }}</span>
          </div>
          <StatusBadge v-if="item.result.response?.statusCode" :code="item.result.response.statusCode" />
          <span class="shrink-0 text-xs tabular-nums text-muted-foreground">{{ item.result.duration }}ms</span>
        </button>
      </div>
    </div>
  </div>
</template>
