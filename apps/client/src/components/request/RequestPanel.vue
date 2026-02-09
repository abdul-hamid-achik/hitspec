<script setup lang="ts">
import { ref } from 'vue'
import UrlBar from './UrlBar.vue'
import HeadersEditor from './HeadersEditor.vue'
import BodyEditor from './BodyEditor.vue'
import AssertionsEditor from './AssertionsEditor.vue'
import { useRequestStore } from '@/stores/request'
import EmptyState from '@/components/common/EmptyState.vue'
import { FileText } from 'lucide-vue-next'

const requestStore = useRequestStore()
const activeTab = ref<'headers' | 'body' | 'assertions' | 'captures'>('headers')
</script>

<template>
  <div class="flex h-full flex-col">
    <template v-if="requestStore.activeRequest">
      <UrlBar />
      <div class="border-b border-border">
        <div class="flex">
          <button
            v-for="tab in (['headers', 'body', 'assertions', 'captures'] as const)"
            :key="tab"
            class="border-b-2 px-4 py-2 text-xs font-medium capitalize transition-colors"
            :class="activeTab === tab
              ? 'border-accent text-foreground'
              : 'border-transparent text-muted-foreground hover:text-foreground'"
            @click="activeTab = tab"
          >
            {{ tab }}
            <span v-if="tab === 'headers'" class="ml-1 text-muted-foreground/60">
              ({{ requestStore.activeRequest.headers?.length ?? 0 }})
            </span>
            <span v-if="tab === 'assertions'" class="ml-1 text-muted-foreground/60">
              ({{ requestStore.activeRequest.assertions?.length ?? 0 }})
            </span>
          </button>
        </div>
      </div>
      <div class="flex-1 overflow-auto p-3">
        <HeadersEditor v-if="activeTab === 'headers'" :headers="requestStore.activeRequest.headers ?? []" />
        <BodyEditor v-else-if="activeTab === 'body'" :body="requestStore.activeRequest.body" />
        <AssertionsEditor v-else-if="activeTab === 'assertions'" :assertions="requestStore.activeRequest.assertions ?? []" />
        <div v-else-if="activeTab === 'captures'" class="space-y-1">
          <div v-for="(cap, i) in requestStore.activeRequest.captures ?? []" :key="i"
            class="flex items-center gap-2 font-mono text-xs">
            <span class="text-nord-14">{{ cap.name }}</span>
            <span class="text-muted-foreground">&larr;</span>
            <span class="text-nord-8">{{ cap.source }}</span>
            <span class="text-muted-foreground">{{ cap.expression }}</span>
          </div>
        </div>
      </div>
    </template>
    <EmptyState v-else :icon="FileText" title="No request selected" description="Select a request from the sidebar to get started" />
  </div>
</template>
