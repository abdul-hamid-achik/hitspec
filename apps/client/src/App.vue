<script setup lang="ts">
import { Toaster } from 'vue-sonner'
import { RouterView } from 'vue-router'
import { useThemeStore } from '@/stores/theme'
import ErrorBoundary from '@/components/common/ErrorBoundary.vue'

const theme = useThemeStore()
</script>

<template>
  <ErrorBoundary>
    <RouterView />
  </ErrorBoundary>
  <Toaster position="bottom-right" :theme="theme.resolved" rich-colors />
</template>
