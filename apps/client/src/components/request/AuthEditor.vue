<script setup lang="ts">
import { computed } from 'vue'
import { useRequestStore } from '@/stores/request'

const requestStore = useRequestStore()

const auth = computed(() => requestStore.activeRequest?.auth ?? {})
const hasAuth = computed(() => Object.keys(auth.value).length > 0)
</script>

<template>
  <div>
    <h3 class="mb-3 text-sm font-medium text-foreground">Authentication</h3>
    <div v-if="hasAuth" class="space-y-2">
      <div
        v-for="(value, key) in auth"
        :key="key"
        class="flex items-center gap-3 rounded-md border border-border bg-nord-0 px-3 py-2"
      >
        <span class="min-w-[100px] text-sm font-medium text-muted-foreground">{{ key }}</span>
        <span class="font-mono text-sm text-foreground">{{ value }}</span>
      </div>
    </div>
    <p v-else class="text-sm text-muted-foreground">
      No authentication configured
    </p>
  </div>
</template>
