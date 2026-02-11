<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { useCollectionStore } from '@/stores/collection'
import { useCodeMirror } from '@/composables/useCodeMirror'
import { Save, Loader2 } from 'lucide-vue-next'

const collection = useCollectionStore()

const editorContainer = ref<HTMLElement | null>(null)
const localContent = computed(() => collection.activeRawContent ?? '')

useCodeMirror(editorContainer, localContent, {
  onChange(value) {
    if (collection.activeFilePath) {
      collection.updateRawContent(collection.activeFilePath, value)
    }
  },
  extraExtensions: () => import('@/editor/index').then(m => ({ default: m.hitspec })),
})

function handleSave() {
  collection.saveActiveFile()
}

function handleKeydown(e: KeyboardEvent) {
  if ((e.metaKey || e.ctrlKey) && e.key === 's') {
    e.preventDefault()
    if (collection.isActiveDirty) {
      handleSave()
    }
  }
}

onMounted(() => {
  window.addEventListener('keydown', handleKeydown)
})

onBeforeUnmount(() => {
  window.removeEventListener('keydown', handleKeydown)
})
</script>

<template>
  <div class="flex h-full flex-col">
    <div class="flex items-center justify-between border-b border-border px-3 py-1.5">
      <div class="flex items-center gap-2">
        <span class="text-xs text-muted-foreground">Source</span>
        <span
          v-if="collection.isActiveDirty"
          class="h-2 w-2 rounded-full bg-nord-13"
          title="Unsaved changes"
        />
      </div>
      <button
        class="flex items-center gap-1.5 rounded-md px-2.5 py-1 text-xs font-medium transition-colors"
        :class="collection.isActiveDirty
          ? 'bg-accent text-accent-foreground hover:bg-accent/90'
          : 'bg-surface text-muted-foreground cursor-not-allowed opacity-50'"
        :disabled="!collection.isActiveDirty || collection.saving"
        @click="handleSave"
      >
        <Loader2 v-if="collection.saving" class="h-3 w-3 animate-spin" />
        <Save v-else class="h-3 w-3" />
        Save
        <kbd v-if="collection.isActiveDirty" class="ml-1 rounded bg-background/50 px-1 py-px text-[10px]">
          {{ navigator.platform?.includes('Mac') ? '\u2318' : 'Ctrl' }}+S
        </kbd>
      </button>
    </div>
    <div ref="editorContainer" class="flex-1 overflow-auto" />
  </div>
</template>
