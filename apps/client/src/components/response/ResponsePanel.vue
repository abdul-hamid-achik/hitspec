<script setup lang="ts">
import { ref } from 'vue'
import { useRequestStore } from '@/stores/request'
import StatusBadge from '@/components/common/StatusBadge.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import ResponseBody from './ResponseBody.vue'
import ResponseHeaders from './ResponseHeaders.vue'
import ResponseAssertions from './ResponseAssertions.vue'
import { CheckCircle, XCircle, Send } from 'lucide-vue-next'

const requestStore = useRequestStore()
const activeTab = ref<'body' | 'headers' | 'assertions'>('body')
</script>

<template>
  <div class="flex h-full flex-col">
    <LoadingSpinner v-if="requestStore.isExecuting" />
    <template v-else-if="requestStore.lastResult">
      <div class="flex items-center gap-3 border-b border-border px-4 py-2">
        <StatusBadge :code="requestStore.lastResult.statusCode" />
        <span class="text-xs text-muted-foreground">{{ requestStore.lastResult.duration }}ms</span>
        <span class="text-xs text-muted-foreground">{{ (requestStore.lastResult.bodySize / 1024).toFixed(1) }}KB</span>
        <div class="flex-1" />
        <component
          :is="requestStore.lastResult.passed ? CheckCircle : XCircle"
          class="h-4 w-4"
          :class="requestStore.lastResult.passed ? 'text-success' : 'text-destructive'"
        />
      </div>
      <div class="border-b border-border">
        <div class="flex">
          <button
            v-for="tab in (['body', 'headers', 'assertions'] as const)"
            :key="tab"
            class="border-b-2 px-4 py-2 text-xs font-medium capitalize transition-colors"
            :class="activeTab === tab
              ? 'border-accent text-foreground'
              : 'border-transparent text-muted-foreground hover:text-foreground'"
            @click="activeTab = tab"
          >
            {{ tab }}
            <span v-if="tab === 'assertions'" class="ml-1 text-muted-foreground/60">
              ({{ requestStore.lastResult.assertions?.length ?? 0 }})
            </span>
          </button>
        </div>
      </div>
      <div class="flex-1 overflow-auto">
        <ResponseBody v-if="activeTab === 'body'" :body="requestStore.lastResult.body" />
        <ResponseHeaders v-else-if="activeTab === 'headers'" :headers="requestStore.lastResult.headers ?? {}" />
        <ResponseAssertions v-else-if="activeTab === 'assertions'" :assertions="requestStore.lastResult.assertions ?? []" />
      </div>
    </template>
    <EmptyState v-else-if="requestStore.error" :icon="XCircle" :title="'Error'" :description="requestStore.error" />
    <EmptyState v-else :icon="Send" title="No response yet" description="Send a request to see the response here" />
  </div>
</template>
