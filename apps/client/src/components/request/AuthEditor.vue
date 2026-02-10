<script setup lang="ts">
import { computed } from 'vue'
import { useRequestStore } from '@/stores/request'

const requestStore = useRequestStore()

const auth = computed(() => requestStore.activeRequest?.metadata?.auth ?? null)
const hasAuth = computed(() => auth.value !== null)
</script>

<template>
  <div>
    <h3 class="mb-3 text-sm font-medium text-foreground">Authentication</h3>
    <div v-if="hasAuth" class="space-y-2">
      <div class="flex items-center gap-3 rounded-md border border-border bg-background px-3 py-2">
        <span class="min-w-[100px] text-sm font-medium text-muted-foreground">Type</span>
        <span class="font-mono text-sm text-foreground">{{ auth!.type }}</span>
      </div>
      <div v-if="auth!.params?.length" class="flex items-center gap-3 rounded-md border border-border bg-background px-3 py-2">
        <span class="min-w-[100px] text-sm font-medium text-muted-foreground">Params</span>
        <span class="font-mono text-sm text-foreground">{{ auth!.params.join(', ') }}</span>
      </div>
    </div>
    <p v-else class="text-sm text-muted-foreground">
      No authentication configured
    </p>
  </div>
</template>
