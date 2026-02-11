<script setup lang="ts">
import { ref } from 'vue'
import UrlBar from './UrlBar.vue'
import HeadersEditor from './HeadersEditor.vue'
import BodyEditor from './BodyEditor.vue'
import AssertionBuilder from './AssertionBuilder.vue'
import SourceEditor from './SourceEditor.vue'
import { useRequestStore } from '@/stores/request'
import { useCollectionStore } from '@/stores/collection'
import EmptyState from '@/components/common/EmptyState.vue'
import { FileText, Globe, AlignLeft, ShieldCheck, Link2, Code } from 'lucide-vue-next'

const requestStore = useRequestStore()
const collection = useCollectionStore()
const activeTab = ref<'headers' | 'body' | 'assertions' | 'captures' | 'source'>('headers')

const tabs = [
  { key: 'headers' as const, label: 'Headers', icon: Globe },
  { key: 'body' as const, label: 'Body', icon: AlignLeft },
  { key: 'assertions' as const, label: 'Assertions', icon: ShieldCheck },
  { key: 'captures' as const, label: 'Captures', icon: Link2 },
  { key: 'source' as const, label: 'Source', icon: Code },
] as const
</script>

<template>
  <div class="flex h-full flex-col">
    <template v-if="requestStore.activeRequest">
      <UrlBar />
      <p v-if="requestStore.activeRequest.description" class="border-b border-border px-3 py-1.5 text-xs text-muted-foreground">
        {{ requestStore.activeRequest.description }}
      </p>
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
              v-if="tab.key === 'headers'"
              class="rounded-full bg-surface px-1.5 py-px text-[10px] tabular-nums text-muted-foreground/60"
            >
              {{ requestStore.activeRequest.headers?.length ?? 0 }}
            </span>
            <span
              v-if="tab.key === 'assertions'"
              class="rounded-full bg-surface px-1.5 py-px text-[10px] tabular-nums text-muted-foreground/60"
            >
              {{ requestStore.activeRequest.assertions?.length ?? 0 }}
            </span>
            <span
              v-if="tab.key === 'source' && collection.isActiveDirty"
              class="h-1.5 w-1.5 rounded-full bg-nord-13"
              title="Unsaved changes"
            />
          </button>
        </div>
      </div>
      <div v-if="activeTab === 'source'" class="flex-1 overflow-hidden">
        <SourceEditor />
      </div>
      <div v-else class="flex-1 overflow-auto p-3">
        <HeadersEditor v-if="activeTab === 'headers'" :headers="requestStore.activeRequest.headers ?? []" />
        <BodyEditor v-else-if="activeTab === 'body'" :body="requestStore.activeRequest.body" />
        <AssertionBuilder
          v-else-if="activeTab === 'assertions'"
          :assertions="requestStore.activeRequest.assertions ?? []"
          @copy="(text: string) => navigator.clipboard.writeText(text).catch(() => {})"
        />
        <div v-else-if="activeTab === 'captures'" class="space-y-1.5">
          <div v-if="!requestStore.activeRequest.captures?.length" class="text-xs text-muted-foreground/60">No captures defined</div>
          <div v-for="(cap, i) in requestStore.activeRequest.captures ?? []" :key="i"
            class="flex items-center gap-2 rounded-md bg-background/50 px-2.5 py-1.5 font-mono text-xs">
            <span class="font-medium text-nord-14">{{ cap.name }}</span>
            <span class="text-muted-foreground/40">&larr;</span>
            <span class="text-nord-8">{{ cap.source }}</span>
            <span class="text-muted-foreground">{{ cap.path }}</span>
          </div>
        </div>
      </div>
    </template>
    <EmptyState
      v-else
      class="flex-1"
      :icon="FileText"
      title="No request selected"
      description="Select a request from the sidebar or use Cmd+K to search"
    />
  </div>
</template>
