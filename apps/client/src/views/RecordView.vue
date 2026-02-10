<script setup lang="ts">
import EmptyState from '@/components/common/EmptyState.vue'
import MethodBadge from '@/components/common/MethodBadge.vue'
import { Play, Square, Trash2, Download, Copy } from 'lucide-vue-next'
import { ref, onMounted } from 'vue'
import { getRecordStatus, startRecording, stopRecording, exportRecordings, clearRecordings } from '@/api/endpoints/record'
import type { RecordStatus } from '@/types/api'
import { toast } from 'vue-sonner'

const status = ref<RecordStatus | null>(null)
const targetUrl = ref('http://localhost:3000')
const port = ref(8081)
const deduplicate = ref(true)
const loading = ref(false)
const exported = ref<string | null>(null)

async function loadStatus() {
  try {
    status.value = await getRecordStatus()
  } catch {
    status.value = { running: false, count: 0 }
  }
}

async function handleStart() {
  if (!targetUrl.value) {
    toast.error('Target URL is required')
    return
  }
  loading.value = true
  try {
    await startRecording({
      targetUrl: targetUrl.value,
      port: port.value,
      deduplicate: deduplicate.value,
    })
    toast.success(`Recording proxy started on port ${port.value}`)
    await loadStatus()
  } catch (e) {
    toast.error(e instanceof Error ? e.message : 'Failed to start')
  } finally {
    loading.value = false
  }
}

async function handleStop() {
  if (!window.confirm('Stop the recording proxy?')) return
  try {
    await stopRecording()
    toast.success('Recording proxy stopped')
    await loadStatus()
  } catch (e) {
    toast.error(e instanceof Error ? e.message : 'Failed to stop')
  }
}

async function handleExport() {
  try {
    const result = await exportRecordings()
    exported.value = result.content
    toast.success('Recordings exported')
  } catch (e) {
    toast.error(e instanceof Error ? e.message : 'Failed to export')
  }
}

async function handleClear() {
  if (!window.confirm('Clear all recordings? This cannot be undone.')) return
  try {
    await clearRecordings()
    await loadStatus()
    toast.success('Recordings cleared')
  } catch (e) {
    toast.error(e instanceof Error ? e.message : 'Failed to clear')
  }
}

async function copyExported() {
  if (exported.value) {
    try {
      await navigator.clipboard.writeText(exported.value)
      toast.success('Copied to clipboard')
    } catch {
      toast.error('Failed to copy to clipboard')
    }
  }
}

onMounted(loadStatus)
</script>

<template>
  <div class="h-full overflow-auto p-6">
      <h1 class="mb-4 text-lg font-semibold text-foreground">Record Proxy</h1>

      <div v-if="status?.running" class="space-y-4">
        <!-- Running status -->
        <div class="rounded-lg border border-border bg-surface p-4 space-y-3">
          <div class="flex items-center gap-2">
            <div class="h-2 w-2 animate-pulse rounded-full bg-success" />
            <span class="text-sm text-foreground">
              Recording on port {{ status.port }} -> {{ status.targetUrl }}
            </span>
          </div>
          <div class="flex items-center gap-2">
            <span class="text-xs text-muted-foreground">{{ status.count }} requests captured</span>
            <div class="ml-auto flex gap-2">
              <button
                class="flex items-center gap-1 rounded bg-accent px-3 py-1 text-xs font-medium text-accent-foreground hover:bg-accent/80"
                @click="handleExport"
              >
                <Download :size="12" /> Export
              </button>
              <button
                class="flex items-center gap-1 rounded bg-surface-hover px-3 py-1 text-xs text-foreground hover:bg-surface"
                @click="handleClear"
              >
                <Trash2 :size="12" /> Clear
              </button>
              <button
                class="flex items-center gap-1 rounded bg-destructive px-3 py-1 text-xs text-foreground hover:bg-destructive/80"
                @click="handleStop"
              >
                <Square :size="12" /> Stop
              </button>
            </div>
          </div>
        </div>

        <!-- Recordings list -->
        <div v-if="status.recordings && status.recordings.length > 0" class="space-y-2">
          <div
            v-for="(rec, i) in status.recordings"
            :key="i"
            class="flex items-center gap-3 rounded border border-border bg-surface p-3"
          >
            <MethodBadge :method="rec.method" size="sm" />
            <span class="flex-1 truncate font-mono text-sm text-foreground">{{ rec.path }}</span>
            <span v-if="rec.statusCode" class="font-mono text-xs" :class="rec.statusCode < 400 ? 'text-success' : 'text-destructive'">
              {{ rec.statusCode }}
            </span>
            <span v-if="rec.duration" class="text-xs text-muted-foreground">{{ rec.duration.toFixed(0) }}ms</span>
          </div>
        </div>

      </div>

      <!-- Exported content (persists after stopping so users don't lose their export) -->
      <div v-if="exported" class="mt-4 rounded-lg border border-border bg-background p-4 space-y-2">
        <div class="flex items-center justify-between">
          <h3 class="text-sm font-medium text-foreground">Exported .http content</h3>
          <button
            class="flex items-center gap-1 text-xs text-accent hover:underline"
            @click="copyExported"
          >
            <Copy :size="12" /> Copy
          </button>
        </div>
        <pre class="max-h-64 overflow-auto rounded bg-background p-3 font-mono text-xs text-foreground">{{ exported }}</pre>
      </div>

      <!-- Not running -->
      <div v-if="!status?.running && !exported" class="space-y-4">
        <div class="rounded-lg border border-border bg-surface p-4 space-y-3">
          <h2 class="text-xs font-semibold uppercase tracking-wider text-muted-foreground/50">Proxy Configuration</h2>
          <div class="grid grid-cols-2 gap-4">
            <div>
              <label class="mb-1 block text-xs text-muted-foreground">Target URL</label>
              <input
                v-model="targetUrl"
                type="text"
                placeholder="http://localhost:3000"
                class="w-full rounded-md border border-border bg-input px-3 py-1.5 text-sm text-foreground focus:outline-none focus:ring-1 focus:ring-ring"
              />
            </div>
            <div>
              <label class="mb-1 block text-xs text-muted-foreground">Proxy Port</label>
              <input
                v-model.number="port"
                type="number"
                min="1"
                class="w-full rounded-md border border-border bg-input px-3 py-1.5 text-sm text-foreground focus:outline-none focus:ring-1 focus:ring-ring"
              />
            </div>
          </div>
          <label class="flex items-center gap-2">
            <input type="checkbox" v-model="deduplicate" class="accent-accent" />
            <span class="text-sm text-muted-foreground">Deduplicate requests</span>
          </label>
        </div>

        <button
          class="flex items-center gap-1.5 rounded-md bg-accent px-4 py-1.5 text-sm font-medium text-accent-foreground hover:bg-accent/80 disabled:opacity-50"
          :disabled="loading"
          @click="handleStart"
        >
          <Play :size="14" />
          {{ loading ? 'Starting...' : 'Start Recording' }}
        </button>

        <p class="text-xs text-muted-foreground/50">
          The recording proxy captures HTTP traffic and exports it as .http files.
          Point your client at the proxy port to start recording.
        </p>
      </div>
  </div>
</template>
