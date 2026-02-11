<script setup lang="ts">
import { onMounted, onBeforeUnmount } from 'vue'
import { Toaster } from 'vue-sonner'
import { RouterView } from 'vue-router'
import { useThemeStore } from '@/stores/theme'
import { useCollectionStore } from '@/stores/collection'
import ErrorBoundary from '@/components/common/ErrorBoundary.vue'

const theme = useThemeStore()
const collection = useCollectionStore()

function handleBeforeUnload(e: BeforeUnloadEvent) {
  if (collection.dirtyFiles.size > 0) {
    e.preventDefault()
  }
}

onMounted(() => window.addEventListener('beforeunload', handleBeforeUnload))
onBeforeUnmount(() => window.removeEventListener('beforeunload', handleBeforeUnload))
</script>

<template>
  <ErrorBoundary>
    <RouterView />
  </ErrorBoundary>
  <Toaster position="bottom-right" :theme="theme.resolved" rich-colors />
</template>
