<script setup lang="ts">
import { computed } from 'vue'
import { useRequestStore } from '@/stores/request'

const requestStore = useRequestStore()

const assertions = computed(() => requestStore.activeRequest?.assertions ?? [])
</script>

<template>
  <div>
    <h3 class="mb-3 text-sm font-medium text-foreground">Assertions</h3>
    <div v-if="assertions.length > 0" class="space-y-1">
      <div
        v-for="(assertion, i) in assertions"
        :key="i"
        class="flex items-center gap-2 rounded-md border border-border bg-nord-0 px-3 py-2 font-mono text-sm"
      >
        <span class="text-accent">{{ assertion.field }}</span>
        <span class="text-nord-15">{{ assertion.operator }}</span>
        <span class="text-nord-14">{{ assertion.expected }}</span>
        <span class="ml-auto text-xs text-nord-3">L{{ assertion.line }}</span>
      </div>
    </div>
    <p v-else class="text-sm text-muted-foreground">
      No assertions defined
    </p>
  </div>
</template>
