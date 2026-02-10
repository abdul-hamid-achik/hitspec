<script setup lang="ts">
import EmptyState from '@/components/common/EmptyState.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import MethodBadge from '@/components/common/MethodBadge.vue'
import {
  History,
  Trash2,
  CheckCircle,
  XCircle,
  Clock,
  AlertCircle,
  ChevronDown,
  ChevronRight,
  ChevronLeft,
  SkipForward,
  Minus,
} from 'lucide-vue-next'
import { useHistoryStore } from '@/stores/history'
import { onMounted, ref } from 'vue'
import dayjs from 'dayjs'
import relativeTime from 'dayjs/plugin/relativeTime'
import type { HistoryResult } from '@/types/api'

dayjs.extend(relativeTime)

const historyStore = useHistoryStore()

// Track which result is expanded to show assertions
const expandedResultId = ref<number | null>(null)

function toggleResult(result: HistoryResult) {
  if (expandedResultId.value === result.id) {
    expandedResultId.value = null
  } else {
    expandedResultId.value = result.id
  }
}

onMounted(() => historyStore.loadRuns())

function confirmClear() {
  if (window.confirm('Clear all history? This cannot be undone.')) {
    historyStore.clearAll()
  }
}

function confirmDeleteRun(id: number, event: Event) {
  event.stopPropagation()
  if (window.confirm('Delete this run? This cannot be undone.')) {
    historyStore.removeRun(id)
  }
}

function formatDuration(ms: number): string {
  if (ms < 1000) return `${ms}ms`
  return `${(ms / 1000).toFixed(1)}s`
}

function fileName(filePath: string): string {
  const parts = filePath.split('/')
  return parts[parts.length - 1]
}
</script>

<template>
  <div class="h-full overflow-auto p-6">
    <div class="mb-4 flex items-center justify-between">
      <h1 class="text-lg font-semibold text-foreground">History</h1>
      <button
        v-if="historyStore.runs.length > 0"
        class="flex items-center gap-1.5 rounded-md border border-border px-2.5 py-1 text-xs text-muted-foreground transition-colors hover:border-destructive/50 hover:text-destructive"
        @click="confirmClear"
      >
        <Trash2 class="h-3 w-3" />
        Clear All
      </button>
    </div>

    <LoadingSpinner v-if="historyStore.loading" label="Loading history..." />

    <div v-else-if="historyStore.error" class="flex flex-col items-center gap-3 p-8">
      <div class="rounded-xl bg-destructive/10 p-3">
        <AlertCircle class="h-8 w-8 text-destructive/60" />
      </div>
      <p class="text-sm text-destructive">{{ historyStore.error }}</p>
      <button
        class="rounded-md border border-border px-3 py-1.5 text-xs text-muted-foreground transition-colors hover:bg-surface-hover hover:text-foreground"
        @click="historyStore.loadRuns()"
      >
        Retry
      </button>
    </div>

    <div v-else-if="historyStore.runs.length > 0" class="space-y-2">
      <!-- Run list -->
      <div v-for="run in historyStore.runs" :key="run.id" class="rounded-lg border border-border bg-surface">
        <!-- Run header (clickable) -->
        <button
          class="flex w-full items-center gap-3 p-3 text-left transition-colors hover:bg-surface-hover"
          @click="historyStore.loadRunDetails(run.id)"
        >
          <component
            :is="historyStore.expandedRunId === run.id ? ChevronDown : ChevronRight"
            class="h-4 w-4 shrink-0 text-muted-foreground/60"
          />

          <span class="flex-1 truncate font-mono text-xs text-foreground/80" :title="run.filePath">
            {{ fileName(run.filePath) }}
          </span>

          <span v-if="run.environment" class="rounded bg-accent/10 px-1.5 py-0.5 text-[10px] text-accent">
            {{ run.environment }}
          </span>

          <!-- Pass/fail/skip counts -->
          <span v-if="run.passed > 0" class="flex items-center gap-0.5 text-xs tabular-nums text-success">
            <CheckCircle class="h-3 w-3" />
            {{ run.passed }}
          </span>
          <span v-if="run.failed > 0" class="flex items-center gap-0.5 text-xs tabular-nums text-destructive">
            <XCircle class="h-3 w-3" />
            {{ run.failed }}
          </span>
          <span v-if="run.skipped > 0" class="flex items-center gap-0.5 text-xs tabular-nums text-muted-foreground">
            <SkipForward class="h-3 w-3" />
            {{ run.skipped }}
          </span>

          <span class="flex items-center gap-1 text-xs tabular-nums text-muted-foreground/50">
            <Clock class="h-3 w-3" />
            {{ formatDuration(run.durationMs) }}
          </span>

          <span class="text-[11px] text-muted-foreground/40" :title="dayjs(run.startedAt).format('YYYY-MM-DD HH:mm:ss')">
            {{ dayjs(run.startedAt).fromNow() }}
          </span>

          <button
            class="rounded p-1 text-muted-foreground/30 transition-colors hover:bg-destructive/10 hover:text-destructive"
            title="Delete run"
            @click="confirmDeleteRun(run.id, $event)"
          >
            <Trash2 class="h-3 w-3" />
          </button>
        </button>

        <!-- Expanded run details -->
        <div v-if="historyStore.expandedRunId === run.id" class="border-t border-border">
          <LoadingSpinner v-if="historyStore.loadingDetails" label="Loading results..." />
          <div v-else-if="historyStore.expandedResults.length === 0" class="p-4 text-center text-xs text-muted-foreground">
            No results recorded
          </div>
          <div v-else class="divide-y divide-border/50">
            <div v-for="result in historyStore.expandedResults" :key="result.id">
              <!-- Result row -->
              <button
                class="flex w-full items-center gap-2 px-4 py-2 text-left transition-colors hover:bg-surface-hover"
                @click="toggleResult(result)"
              >
                <component
                  :is="result.skipped ? Minus : result.passed ? CheckCircle : XCircle"
                  class="h-3.5 w-3.5 shrink-0"
                  :class="
                    result.skipped
                      ? 'text-muted-foreground/40'
                      : result.passed
                        ? 'text-success/60'
                        : 'text-destructive/60'
                  "
                />
                <MethodBadge :method="result.method" size="sm" />
                <span class="flex-1 truncate font-mono text-xs text-foreground/70">
                  {{ result.requestName }}
                </span>
                <span v-if="result.statusCode" class="font-mono text-[11px] tabular-nums" :class="
                  result.statusCode < 300
                    ? 'text-success/70'
                    : result.statusCode < 400
                      ? 'text-warning/70'
                      : 'text-destructive/70'
                ">
                  {{ result.statusCode }}
                </span>
                <span class="text-[11px] tabular-nums text-muted-foreground/40">
                  {{ formatDuration(result.durationMs) }}
                </span>
                <component
                  :is="expandedResultId === result.id ? ChevronDown : ChevronRight"
                  v-if="result.assertions && result.assertions.length > 0"
                  class="h-3 w-3 text-muted-foreground/30"
                />
              </button>

              <!-- Error message -->
              <div v-if="result.error" class="mx-4 mb-2 rounded bg-destructive/5 px-3 py-1.5 text-xs text-destructive">
                {{ result.error }}
              </div>

              <!-- Expanded assertions -->
              <div
                v-if="expandedResultId === result.id && result.assertions && result.assertions.length > 0"
                class="mx-4 mb-2 rounded border border-border/50 bg-background"
              >
                <div
                  v-for="assertion in result.assertions"
                  :key="assertion.id"
                  class="flex items-center gap-2 border-b border-border/30 px-3 py-1.5 text-[11px] last:border-0"
                >
                  <component
                    :is="assertion.passed ? CheckCircle : XCircle"
                    class="h-3 w-3 shrink-0"
                    :class="assertion.passed ? 'text-success/50' : 'text-destructive/50'"
                  />
                  <span class="font-mono text-muted-foreground">{{ assertion.subject }}</span>
                  <span class="text-foreground/50">{{ assertion.operator }}</span>
                  <span v-if="assertion.expected" class="font-mono text-foreground/60">{{ assertion.expected }}</span>
                  <span v-if="!assertion.passed && assertion.actual" class="ml-auto font-mono text-destructive/70">
                    got: {{ assertion.actual }}
                  </span>
                  <span v-if="assertion.message" class="ml-auto text-muted-foreground/50">{{ assertion.message }}</span>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- Pagination -->
      <div v-if="historyStore.totalRuns > 20" class="flex items-center justify-between pt-2">
        <button
          :disabled="!historyStore.hasPrevPage()"
          class="flex items-center gap-1 rounded-md border border-border px-2.5 py-1 text-xs text-muted-foreground transition-colors hover:bg-surface-hover disabled:opacity-30 disabled:pointer-events-none"
          @click="historyStore.prevPage()"
        >
          <ChevronLeft class="h-3 w-3" />
          Previous
        </button>
        <span class="text-xs text-muted-foreground/50">
          Page {{ historyStore.currentPage + 1 }} of {{ Math.ceil(historyStore.totalRuns / 20) }}
        </span>
        <button
          :disabled="!historyStore.hasNextPage()"
          class="flex items-center gap-1 rounded-md border border-border px-2.5 py-1 text-xs text-muted-foreground transition-colors hover:bg-surface-hover disabled:opacity-30 disabled:pointer-events-none"
          @click="historyStore.nextPage()"
        >
          Next
          <ChevronRight class="h-3 w-3" />
        </button>
      </div>
    </div>

    <EmptyState v-else :icon="History" title="No history yet" description="Execute requests to see them here" />
  </div>
</template>
