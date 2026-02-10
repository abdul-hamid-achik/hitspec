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
import { CheckCircle, XCircle, Send } from 'lucide-vue-next'
import type { RunResult } from '@/types/api'

const requestStore = useRequestStore()
const activeTab = ref<'body' | 'headers' | 'assertions' | 'results'>('body')

const hasRunResults = computed(() =>
  requestStore.lastRunResult !== null && requestStore.lastRunResult.results.length > 0
)

const tabs = computed(() => {
  const base = ['body', 'headers', 'assertions'] as const
  if (hasRunResults.value) {
    return ['results', ...base] as const
  }
  return base
})

// Auto-switch to Results tab when a Run All completes
watch(() => requestStore.lastRunResult, (newVal) => {
  if (newVal && newVal.results.length > 0) {
    activeTab.value = 'results'
  }
})

function selectResult(result: RunResult) {
  requestStore.lastResult = result
  activeTab.value = 'body'
}
</script>

<template>
  <div class="flex h-full flex-col">
    <LoadingSpinner v-if="requestStore.isExecuting" />
    <template v-else-if="requestStore.lastResult || hasRunResults">
      <div class="flex items-center gap-3 border-b border-border px-4 py-2">
        <StatusBadge v-if="requestStore.lastResult?.response?.statusCode" :code="requestStore.lastResult.response.statusCode" />
        <span v-if="requestStore.lastResult" class="text-xs text-muted-foreground">{{ requestStore.lastResult.response?.duration ?? requestStore.lastResult.duration }}ms</span>
        <span v-if="requestStore.lastResult" class="text-xs text-muted-foreground">{{ ((requestStore.lastResult.response?.size ?? 0) / 1024).toFixed(1) }}KB</span>
        <div class="flex-1" />
        <component
          v-if="requestStore.lastResult"
          :is="requestStore.lastResult.passed ? CheckCircle : XCircle"
          class="h-4 w-4"
          :class="requestStore.lastResult.passed ? 'text-success' : 'text-destructive'"
        />
      </div>
      <div class="border-b border-border">
        <div class="flex">
          <button
            v-for="tab in tabs"
            :key="tab"
            class="border-b-2 px-4 py-2 text-xs font-medium capitalize transition-colors"
            :class="activeTab === tab
              ? 'border-accent text-foreground'
              : 'border-transparent text-muted-foreground hover:text-foreground'"
            @click="activeTab = tab"
          >
            {{ tab }}
            <span v-if="tab === 'assertions' && requestStore.lastResult" class="ml-1 text-muted-foreground/60">
              ({{ requestStore.lastResult.assertions?.length ?? 0 }})
            </span>
            <span v-if="tab === 'results' && requestStore.lastRunResult" class="ml-1 text-muted-foreground/60">
              ({{ requestStore.lastRunResult.results.length }})
            </span>
          </button>
        </div>
      </div>
      <div class="flex-1 overflow-auto">
        <RunResultsList
          v-if="activeTab === 'results' && requestStore.lastRunResult"
          :results="requestStore.lastRunResult"
          @select="selectResult"
        />
        <template v-else-if="requestStore.lastResult">
          <ResponseBody v-if="activeTab === 'body'" :body="requestStore.lastResult.response?.body" />
          <ResponseHeaders v-else-if="activeTab === 'headers'" :headers="requestStore.lastResult.response?.headers ?? {}" />
          <ResponseAssertions v-else-if="activeTab === 'assertions'" :assertions="requestStore.lastResult.assertions ?? []" />
        </template>
      </div>
    </template>
    <EmptyState v-else-if="requestStore.error" :icon="XCircle" :title="'Error'" :description="requestStore.error" />
    <EmptyState v-else :icon="Send" title="No response yet" description="Send a request to see the response here" />
  </div>
</template>
