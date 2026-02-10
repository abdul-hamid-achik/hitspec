<script setup lang="ts">
import { ref, watch, computed } from 'vue'
import { useRequestStore } from '@/stores/request'
import StatusBadge from '@/components/common/StatusBadge.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import ResponseBody from './ResponseBody.vue'
import ResponseHeaders from './ResponseHeaders.vue'
import ResponseAssertions from './ResponseAssertions.vue'
import RunResultsList from './RunResultsList.vue'
import {
  CheckCircle, XCircle, MinusCircle, Send, AlignLeft, Globe, ShieldCheck, List,
  Timer, HardDrive,
} from 'lucide-vue-next'
import type { RunResult } from '@/types/api'

const requestStore = useRequestStore()
const activeTab = ref<'body' | 'headers' | 'assertions' | 'results'>('body')

const hasRunResults = computed(() =>
  requestStore.lastRunResult !== null && requestStore.lastRunResult.results.length > 0
)

const tabs = computed(() => {
  const base = [
    { key: 'body' as const, label: 'Body', icon: AlignLeft },
    { key: 'headers' as const, label: 'Headers', icon: Globe },
    { key: 'assertions' as const, label: 'Assertions', icon: ShieldCheck },
  ]
  if (hasRunResults.value) {
    return [{ key: 'results' as const, label: 'Results', icon: List }, ...base]
  }
  return base
})

// Auto-switch to Results tab when a Run All completes
watch(() => requestStore.lastRunResult, (newVal) => {
  if (newVal && newVal.results.length > 0) {
    activeTab.value = 'results'
  } else if (!newVal && activeTab.value === 'results') {
    activeTab.value = 'body'
  }
})

function selectResult(result: RunResult) {
  requestStore.lastResult = result
  activeTab.value = 'body'
}
</script>

<template>
  <div class="flex h-full flex-col">
    <!-- Loading / Progress state -->
    <div v-if="requestStore.isExecuting" class="flex flex-1 flex-col items-center justify-center gap-4 p-6">
      <template v-if="requestStore.executionProgress">
        <div class="w-full max-w-md space-y-4">
          <!-- Progress bar -->
          <div class="space-y-1.5">
            <div class="flex items-center justify-between text-xs text-muted-foreground">
              <span>{{ requestStore.executionProgress.completed }}/{{ requestStore.executionProgress.total }} requests</span>
              <span>{{ Math.round((requestStore.executionProgress.completed / requestStore.executionProgress.total) * 100) }}%</span>
            </div>
            <div class="h-1.5 w-full overflow-hidden rounded-full bg-surface">
              <div
                class="h-full rounded-full bg-accent transition-all duration-300"
                :style="{ width: `${(requestStore.executionProgress.completed / requestStore.executionProgress.total) * 100}%` }"
              />
            </div>
          </div>
          <!-- Current request -->
          <div class="flex items-center gap-2 text-sm">
            <div class="h-4 w-4 animate-spin rounded-full border-2 border-accent/30 border-t-accent" />
            <span class="truncate text-foreground/80">{{ requestStore.executionProgress.currentRequest }}</span>
          </div>
          <!-- Completed results -->
          <div v-if="requestStore.executionProgress.results.length > 0" class="max-h-48 space-y-1 overflow-y-auto">
            <div
              v-for="(r, i) in requestStore.executionProgress.results"
              :key="i"
              class="flex items-center gap-2 rounded px-2 py-1 text-xs"
              :class="r.passed ? 'text-success/80' : 'text-destructive/80'"
            >
              <CheckCircle v-if="r.passed" class="h-3 w-3 shrink-0" />
              <XCircle v-else class="h-3 w-3 shrink-0" />
              <span class="flex-1 truncate">{{ r.name }}</span>
              <span class="tabular-nums text-muted-foreground/50">{{ r.duration }}ms</span>
            </div>
          </div>
        </div>
      </template>
      <LoadingSpinner v-else size="lg" label="Executing request..." />
    </div>

    <!-- Response content -->
    <template v-else-if="requestStore.lastResult || hasRunResults">
      <!-- Status bar -->
      <div class="flex items-center gap-2 border-b border-border px-4 py-2">
        <StatusBadge v-if="requestStore.lastResult?.response?.statusCode" :code="requestStore.lastResult.response.statusCode" />
        <span v-else-if="requestStore.lastResult?.error" class="rounded bg-destructive/10 px-1.5 py-0.5 text-[10px] font-medium text-destructive">ERROR</span>
        <span v-else-if="requestStore.lastResult?.skipped" class="rounded bg-muted px-1.5 py-0.5 text-[10px] font-medium text-muted-foreground">SKIPPED</span>
        <div class="flex items-center gap-3 text-xs text-muted-foreground/60">
          <span v-if="requestStore.lastResult" class="flex items-center gap-1">
            <Timer class="h-3 w-3" />
            {{ requestStore.lastResult.response?.duration ?? requestStore.lastResult.duration }}ms
          </span>
          <span v-if="requestStore.lastResult?.response?.size" class="flex items-center gap-1">
            <HardDrive class="h-3 w-3" />
            {{ ((requestStore.lastResult.response.size) / 1024).toFixed(1) }}KB
          </span>
          <span v-if="requestStore.lastResult?.skipReason" class="text-muted-foreground">
            {{ requestStore.lastResult.skipReason }}
          </span>
        </div>
        <div class="flex-1" />
        <div v-if="requestStore.lastResult" class="flex items-center gap-1 text-xs font-medium">
          <CheckCircle v-if="requestStore.lastResult.passed" class="h-3.5 w-3.5 text-success" />
          <XCircle v-else-if="!requestStore.lastResult.skipped" class="h-3.5 w-3.5 text-destructive" />
          <MinusCircle v-else class="h-3.5 w-3.5 text-muted-foreground" />
          <span :class="requestStore.lastResult.passed ? 'text-success' : requestStore.lastResult.skipped ? 'text-muted-foreground' : 'text-destructive'">
            {{ requestStore.lastResult.passed ? 'Passed' : requestStore.lastResult.skipped ? 'Skipped' : 'Failed' }}
          </span>
        </div>
      </div>

      <!-- Tabs -->
      <div class="border-b border-border">
        <div class="flex gap-0.5 px-2">
          <button
            v-for="tab in tabs"
            :key="tab.key"
            class="flex items-center gap-1.5 border-b-2 px-3 py-2 text-xs font-medium transition-colors"
            :class="activeTab === tab.key
              ? 'border-accent text-foreground'
              : 'border-transparent text-muted-foreground hover:text-foreground'"
            @click="activeTab = tab.key"
          >
            <component :is="tab.icon" class="h-3 w-3" />
            {{ tab.label }}
            <span
              v-if="tab.key === 'assertions' && requestStore.lastResult"
              class="rounded-full bg-surface px-1.5 py-px text-[10px] tabular-nums text-muted-foreground/60"
            >
              {{ requestStore.lastResult.assertions?.length ?? 0 }}
            </span>
            <span
              v-if="tab.key === 'results' && requestStore.lastRunResult"
              class="rounded-full bg-surface px-1.5 py-px text-[10px] tabular-nums text-muted-foreground/60"
            >
              {{ requestStore.lastRunResult.results.length }}
            </span>
          </button>
        </div>
      </div>

      <!-- Tab content -->
      <div class="flex-1 overflow-auto">
        <RunResultsList
          v-if="activeTab === 'results' && requestStore.lastRunResult"
          :results="requestStore.lastRunResult"
          @select="selectResult"
        />
        <template v-else-if="requestStore.lastResult">
          <ResponseBody v-if="activeTab === 'body'" :body="requestStore.lastResult.response?.body" :error="requestStore.lastResult.error" />
          <ResponseHeaders v-else-if="activeTab === 'headers'" :headers="requestStore.lastResult.response?.headers ?? {}" />
          <ResponseAssertions v-else-if="activeTab === 'assertions'" :assertions="requestStore.lastResult.assertions ?? []" />
        </template>
      </div>
    </template>

    <!-- Error state -->
    <div v-else-if="requestStore.error" class="flex flex-1 flex-col items-center justify-center gap-3 p-8">
      <div class="rounded-xl bg-destructive/10 p-3">
        <XCircle class="h-10 w-10 text-destructive/60" />
      </div>
      <div class="space-y-1 text-center">
        <h3 class="text-sm font-medium text-foreground">Request Failed</h3>
        <p class="max-w-sm text-xs text-muted-foreground">{{ requestStore.error }}</p>
      </div>
      <button
        class="rounded-md border border-border px-3 py-1.5 text-xs text-muted-foreground transition-colors hover:bg-surface-hover hover:text-foreground"
        @click="requestStore.error = null"
      >
        Dismiss
      </button>
    </div>

    <!-- Empty state -->
    <EmptyState v-else class="flex-1" :icon="Send" title="No response yet" description="Send a request or press Cmd+Enter to run the file" />
  </div>
</template>
