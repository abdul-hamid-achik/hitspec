<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useComparisonStore } from '@/stores/comparison'
import { useCollectionStore } from '@/stores/collection'
import { useRequestStore } from '@/stores/request'
import { useDiffWorker } from '@/composables/useDiffWorker'
import { formatUnifiedDiff, type DiffLine } from '@/lib/diff'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import StatusBadge from '@/components/common/StatusBadge.vue'
import {
  CheckCircle, XCircle, MinusCircle, Loader2, Copy, Check,
  AlignJustify, Columns2, GitCompareArrows,
} from 'lucide-vue-next'
import { toast } from 'vue-sonner'
import dayjs from 'dayjs'
import relativeTime from 'dayjs/plugin/relativeTime'
import type { HistoryResultWithRun } from '@/types/api'

dayjs.extend(relativeTime)

const comparison = useComparisonStore()
const collection = useCollectionStore()
const requestStore = useRequestStore()

const { compute: computeDiffAsync, computing } = useDiffWorker()
const diffLines = ref<DiffLine[]>([])
const viewMode = ref<'unified' | 'split'>('unified')
const copied = ref(false)

// Auto-load history when the active request changes
watch(
  [() => requestStore.activeRequest?.name, () => collection.activeFilePath],
  ([name, file]) => {
    if (name && file) {
      comparison.loadHistory(name, file)
    }
  },
  { immediate: true },
)

// Recompute diff when selection changes
watch(
  [() => comparison.selectedA, () => comparison.selectedB],
  async ([a, b]) => {
    if (a && b) {
      diffLines.value = []
      diffLines.value = await computeDiffAsync(a.bodyPreview ?? '', b.bodyPreview ?? '')
    } else {
      diffLines.value = []
    }
  },
)

const stats = computed(() => {
  let added = 0
  let removed = 0
  for (const l of diffLines.value) {
    if (l.type === 'add') added++
    else if (l.type === 'remove') removed++
  }
  return { added, removed }
})

const splitPairs = computed(() => {
  const pairs: Array<{ left: DiffLine | null; right: DiffLine | null }> = []
  const lines = diffLines.value
  let i = 0
  while (i < lines.length) {
    const line = lines[i]
    if (line.type === 'equal') {
      pairs.push({ left: line, right: line })
      i++
    } else if (line.type === 'remove') {
      const removes: DiffLine[] = []
      while (i < lines.length && lines[i].type === 'remove') {
        removes.push(lines[i])
        i++
      }
      const adds: DiffLine[] = []
      while (i < lines.length && lines[i].type === 'add') {
        adds.push(lines[i])
        i++
      }
      const max = Math.max(removes.length, adds.length)
      for (let j = 0; j < max; j++) {
        pairs.push({
          left: j < removes.length ? removes[j] : null,
          right: j < adds.length ? adds[j] : null,
        })
      }
    } else {
      pairs.push({ left: null, right: line })
      i++
    }
  }
  return pairs
})

function isSelected(result: HistoryResultWithRun): 'a' | 'b' | false {
  if (comparison.selectedA?.id === result.id) return 'a'
  if (comparison.selectedB?.id === result.id) return 'b'
  return false
}

function resultIcon(result: HistoryResultWithRun) {
  if (result.skipped) return MinusCircle
  return result.passed ? CheckCircle : XCircle
}

function resultIconClass(result: HistoryResultWithRun) {
  if (result.skipped) return 'text-muted-foreground'
  return result.passed ? 'text-success' : 'text-destructive'
}

async function copyDiff() {
  if (!comparison.selectedA || !comparison.selectedB) return
  try {
    const text = formatUnifiedDiff(
      diffLines.value,
      `${comparison.selectedA.method} (${dayjs(comparison.selectedA.runStartedAt).format('HH:mm:ss')})`,
      `${comparison.selectedB.method} (${dayjs(comparison.selectedB.runStartedAt).format('HH:mm:ss')})`,
    )
    await navigator.clipboard.writeText(text)
    copied.value = true
    setTimeout(() => (copied.value = false), 2000)
  } catch {
    toast.error('Failed to copy to clipboard')
  }
}

function lineClass(type: DiffLine['type']): string {
  if (type === 'add') return 'bg-success/10 text-success'
  if (type === 'remove') return 'bg-destructive/10 text-destructive'
  return 'text-foreground/80'
}

function gutterClass(type: DiffLine['type']): string {
  if (type === 'add') return 'bg-success/15 text-success/60'
  if (type === 'remove') return 'bg-destructive/15 text-destructive/60'
  return 'text-muted-foreground/50'
}

function prefix(type: DiffLine['type']): string {
  if (type === 'add') return '+'
  if (type === 'remove') return '-'
  return ' '
}
</script>

<template>
  <div class="flex h-full">
    <!-- Left sidebar: historical runs -->
    <div class="flex w-52 shrink-0 flex-col border-r border-border">
      <div class="border-b border-border px-3 py-2">
        <h3 class="text-[11px] font-semibold uppercase tracking-wider text-muted-foreground/60">Previous Runs</h3>
      </div>

      <div v-if="comparison.loading" class="flex items-center justify-center p-4">
        <LoadingSpinner label="Loading..." />
      </div>

      <div v-else-if="comparison.requestHistory.length === 0" class="p-4 text-center text-xs text-muted-foreground">
        No previous runs found
      </div>

      <div v-else class="flex-1 overflow-y-auto">
        <button
          v-for="result in comparison.requestHistory"
          :key="result.id"
          class="flex w-full items-center gap-2 border-b border-border/30 px-3 py-2 text-left text-xs transition-colors hover:bg-surface-hover"
          :class="{
            'bg-accent/10 ring-1 ring-inset ring-accent/30': isSelected(result),
          }"
          @click="comparison.selectForCompare(result)"
        >
          <component :is="resultIcon(result)" class="h-3 w-3 shrink-0" :class="resultIconClass(result)" />
          <div class="min-w-0 flex-1">
            <div class="flex items-center gap-1">
              <StatusBadge v-if="result.statusCode" :code="result.statusCode" />
              <span class="tabular-nums text-muted-foreground">{{ result.durationMs }}ms</span>
            </div>
            <div class="truncate text-[10px] text-muted-foreground/50" :title="dayjs(result.runStartedAt).format('YYYY-MM-DD HH:mm:ss')">
              {{ dayjs(result.runStartedAt).fromNow() }}
            </div>
          </div>
          <span
            v-if="isSelected(result)"
            class="shrink-0 rounded bg-accent/20 px-1 text-[9px] font-bold text-accent"
          >
            {{ isSelected(result) === 'a' ? 'A' : 'B' }}
          </span>
        </button>
      </div>

      <div v-if="comparison.selectedA || comparison.selectedB" class="border-t border-border px-3 py-2">
        <button
          class="w-full text-[11px] text-muted-foreground hover:text-foreground"
          @click="comparison.clearSelection()"
        >
          Clear selection
        </button>
      </div>
    </div>

    <!-- Right content: diff view -->
    <div class="flex flex-1 flex-col overflow-hidden">
      <template v-if="comparison.hasComparison">
        <!-- Diff toolbar -->
        <div class="flex items-center justify-between border-b border-border px-4 py-2">
          <div class="flex items-center gap-2">
            <div class="flex gap-1">
              <button
                class="flex items-center gap-1.5 rounded-md px-2.5 py-1 text-xs font-medium transition-colors"
                :class="viewMode === 'unified' ? 'bg-accent/15 text-accent' : 'text-muted-foreground hover:bg-surface-hover hover:text-foreground'"
                @click="viewMode = 'unified'"
              >
                <AlignJustify class="h-3 w-3" />
                Unified
              </button>
              <button
                class="flex items-center gap-1.5 rounded-md px-2.5 py-1 text-xs font-medium transition-colors"
                :class="viewMode === 'split' ? 'bg-accent/15 text-accent' : 'text-muted-foreground hover:bg-surface-hover hover:text-foreground'"
                @click="viewMode = 'split'"
              >
                <Columns2 class="h-3 w-3" />
                Split
              </button>
            </div>
            <span v-if="stats.added > 0" class="text-[11px] font-medium text-success">+{{ stats.added }}</span>
            <span v-if="stats.removed > 0" class="text-[11px] font-medium text-destructive">-{{ stats.removed }}</span>
          </div>
          <button
            :disabled="computing"
            class="flex items-center gap-1 rounded-md border border-border bg-surface px-2 py-1 text-xs text-muted-foreground transition-colors hover:bg-surface-hover hover:text-foreground disabled:opacity-50"
            @click="copyDiff"
          >
            <Check v-if="copied" class="h-3 w-3 text-success" />
            <Copy v-else class="h-3 w-3" />
            {{ copied ? 'Copied!' : 'Copy diff' }}
          </button>
        </div>

        <!-- Diff content -->
        <div class="flex-1 overflow-auto">
          <div v-if="computing" class="flex items-center justify-center gap-2 p-8">
            <Loader2 class="h-5 w-5 animate-spin text-accent" />
            <span class="text-sm text-muted-foreground">Computing diff...</span>
          </div>

          <!-- Unified view -->
          <div v-else-if="viewMode === 'unified'" class="font-mono text-xs leading-relaxed">
            <div v-if="diffLines.length === 0" class="p-8 text-center text-sm text-muted-foreground">
              Responses are identical
            </div>
            <div
              v-for="(line, i) in diffLines"
              :key="i"
              class="flex"
              :class="lineClass(line.type)"
            >
              <span class="w-10 shrink-0 select-none px-2 py-px text-right" :class="gutterClass(line.type)">{{ line.oldLineNum ?? '' }}</span>
              <span class="w-10 shrink-0 select-none px-2 py-px text-right" :class="gutterClass(line.type)">{{ line.newLineNum ?? '' }}</span>
              <span class="shrink-0 select-none px-1 py-px" :class="gutterClass(line.type)">{{ prefix(line.type) }}</span>
              <span class="flex-1 whitespace-pre-wrap break-all px-2 py-px">{{ line.content }}</span>
            </div>
          </div>

          <!-- Split view -->
          <div v-else class="font-mono text-xs leading-relaxed">
            <div v-if="splitPairs.length === 0" class="p-8 text-center text-sm text-muted-foreground">
              Responses are identical
            </div>
            <div v-for="(pair, i) in splitPairs" :key="i" class="flex">
              <div
                class="flex flex-1 min-w-0"
                :class="pair.left ? lineClass(pair.left.type === 'equal' ? 'equal' : 'remove') : ''"
              >
                <span class="w-10 shrink-0 select-none px-2 py-px text-right" :class="pair.left ? gutterClass(pair.left.type === 'equal' ? 'equal' : 'remove') : 'text-muted-foreground/20'">{{ pair.left?.oldLineNum ?? '' }}</span>
                <span class="flex-1 whitespace-pre-wrap break-all px-2 py-px">{{ pair.left?.content ?? '' }}</span>
              </div>
              <div
                class="flex flex-1 min-w-0 border-l border-border/50"
                :class="pair.right ? lineClass(pair.right.type === 'equal' ? 'equal' : 'add') : ''"
              >
                <span class="w-10 shrink-0 select-none px-2 py-px text-right" :class="pair.right ? gutterClass(pair.right.type === 'equal' ? 'equal' : 'add') : 'text-muted-foreground/20'">{{ pair.right?.newLineNum ?? '' }}</span>
                <span class="flex-1 whitespace-pre-wrap break-all px-2 py-px">{{ pair.right?.content ?? '' }}</span>
              </div>
            </div>
          </div>
        </div>
      </template>

      <!-- Empty state: no comparison selected -->
      <div v-else class="flex flex-1 flex-col items-center justify-center gap-3 p-8">
        <div class="rounded-xl bg-accent/10 p-3">
          <GitCompareArrows class="h-8 w-8 text-accent/60" />
        </div>
        <div class="space-y-1 text-center">
          <h3 class="text-sm font-medium text-foreground">Compare Responses</h3>
          <p class="max-w-xs text-xs text-muted-foreground">
            Select two runs from the sidebar to compare their response bodies
          </p>
        </div>
      </div>
    </div>
  </div>
</template>
