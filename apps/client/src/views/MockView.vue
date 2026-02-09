<script setup lang="ts">
import AppShell from '@/components/layout/AppShell.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import MethodBadge from '@/components/common/MethodBadge.vue'
import { Server } from 'lucide-vue-next'
import { ref, onMounted } from 'vue'
import { getMockStatus, startMock, stopMock, type MockStatus } from '@/api/endpoints/mock'

const status = ref<MockStatus | null>(null)

async function loadStatus() {
  status.value = await getMockStatus()
}

async function handleStart() {
  await startMock({ files: [], port: 8080 })
  await loadStatus()
}

async function handleStop() {
  await stopMock()
  await loadStatus()
}

onMounted(loadStatus)
</script>

<template>
  <AppShell>
    <div class="p-6">
      <h1 class="mb-4 text-lg font-semibold text-foreground">Mock Server</h1>
      <div v-if="status?.running" class="space-y-4">
        <div class="flex items-center gap-2">
          <div class="h-2 w-2 animate-pulse rounded-full bg-success" />
          <span class="text-sm text-foreground">Running on port {{ status.port }}</span>
          <button
            class="ml-4 rounded bg-destructive px-3 py-1 text-sm text-foreground hover:bg-destructive/80"
            @click="handleStop"
          >
            Stop
          </button>
        </div>
        <div class="space-y-2">
          <div
            v-for="(route, i) in status.routes"
            :key="i"
            class="flex items-center gap-3 rounded border border-border bg-surface p-3"
          >
            <MethodBadge :method="route.method" />
            <span class="font-mono text-sm text-foreground">{{ route.path }}</span>
            <span class="text-xs text-muted-foreground">-> {{ route.status }}</span>
          </div>
        </div>
      </div>
      <EmptyState v-else :icon="Server" title="Mock server not running" description="Start a mock server from your .http files">
        <button
          class="mt-2 rounded bg-accent px-4 py-2 text-sm font-medium text-accent-foreground hover:bg-accent/80"
          @click="handleStart"
        >
          Start Mock Server
        </button>
      </EmptyState>
    </div>
  </AppShell>
</template>
