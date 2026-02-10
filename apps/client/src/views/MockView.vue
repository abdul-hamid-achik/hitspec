<script setup lang="ts">
import AppShell from '@/components/layout/AppShell.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import MethodBadge from '@/components/common/MethodBadge.vue'
import { Server, Square } from 'lucide-vue-next'
import { ref, onMounted } from 'vue'
import { getMockStatus, startMock, stopMock } from '@/api/endpoints/mock'
import type { MockStatusDTO } from '@/types/api'
import { toast } from 'vue-sonner'

const status = ref<MockStatusDTO | null>(null)
const mockPort = ref(8080)
const starting = ref(false)

async function loadStatus() {
  try {
    status.value = await getMockStatus()
  } catch {
    status.value = null
  }
}

async function handleStart() {
  starting.value = true
  try {
    await startMock({ files: [], port: mockPort.value })
    toast.success(`Mock server started on port ${mockPort.value}`)
    await loadStatus()
  } catch (e) {
    toast.error(e instanceof Error ? e.message : 'Failed to start mock server')
  } finally {
    starting.value = false
  }
}

async function handleStop() {
  if (!window.confirm('Stop the mock server?')) return
  try {
    await stopMock()
    toast.success('Mock server stopped')
    await loadStatus()
  } catch (e) {
    toast.error(e instanceof Error ? e.message : 'Failed to stop mock server')
  }
}

onMounted(loadStatus)
</script>

<template>
  <AppShell>
    <div class="h-full overflow-auto p-6">
      <div class="mb-4 flex items-center justify-between">
        <h1 class="text-lg font-semibold text-foreground">Mock Server</h1>
        <button
          v-if="status?.running"
          class="flex items-center gap-1.5 rounded-md border border-destructive/50 px-3 py-1 text-xs font-medium text-destructive transition-colors hover:bg-destructive/10"
          @click="handleStop"
        >
          <Square class="h-3 w-3" />
          Stop Server
        </button>
      </div>

      <div v-if="status?.running" class="space-y-4">
        <!-- Running indicator -->
        <div class="flex items-center gap-3 rounded-lg border border-success/20 bg-success/5 p-3">
          <div class="h-2 w-2 animate-pulse rounded-full bg-success" />
          <span class="text-sm font-medium text-foreground">Running</span>
          <span class="rounded-md bg-surface px-2 py-0.5 font-mono text-xs text-accent">
            localhost:{{ status.port }}
          </span>
          <span class="text-xs text-muted-foreground/50">{{ status.routes?.length ?? 0 }} routes</span>
        </div>

        <!-- Routes -->
        <div v-if="status.routes?.length" class="space-y-1.5">
          <h2 class="text-xs font-semibold uppercase tracking-wider text-muted-foreground/50">Registered Routes</h2>
          <div
            v-for="(route, i) in status.routes"
            :key="i"
            class="flex items-center gap-3 rounded-lg border border-border bg-surface p-3 transition-colors hover:bg-surface-hover"
          >
            <MethodBadge :method="route.method" size="sm" />
            <span class="flex-1 font-mono text-xs text-foreground/80">{{ route.path }}</span>
            <span class="rounded-md bg-background/50 px-2 py-0.5 font-mono text-[11px] tabular-nums text-muted-foreground">
              {{ route.statusCode }}
            </span>
            <span v-if="route.contentType" class="text-[11px] text-muted-foreground/50">
              {{ route.contentType }}
            </span>
          </div>
        </div>
      </div>

      <div v-else class="space-y-4">
        <EmptyState :icon="Server" title="Mock server not running" description="Start a mock server from your .http files" />
        <div class="mx-auto flex max-w-xs items-center gap-3">
          <label class="text-xs text-muted-foreground">Port</label>
          <input
            v-model.number="mockPort"
            type="number"
            min="1"
            class="w-24 rounded-md border border-border bg-background px-2.5 py-1 text-sm tabular-nums text-foreground focus:outline-none focus:ring-1 focus:ring-ring"
          />
          <button
            class="rounded-md bg-accent px-4 py-1.5 text-xs font-medium text-accent-foreground transition-colors hover:bg-accent/80 disabled:opacity-50"
            :disabled="starting"
            @click="handleStart"
          >
            {{ starting ? 'Starting...' : 'Start' }}
          </button>
        </div>
      </div>
    </div>
  </AppShell>
</template>
