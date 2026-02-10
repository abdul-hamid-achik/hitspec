<script setup lang="ts">
import EmptyState from '@/components/common/EmptyState.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import { FileCheck, Play, CheckCircle, XCircle, AlertCircle } from 'lucide-vue-next'
import { ref, onMounted } from 'vue'
import { getContractFiles, verifyContracts } from '@/api/endpoints/contract'
import type { ContractResult } from '@/types/api'
import { toast } from 'vue-sonner'

const files = ref<string[]>([])
const selectedFiles = ref<string[]>([])
const providerUrl = ref('')
const stateHandler = ref('')
const results = ref<ContractResult[]>([])
const loading = ref(false)
const verified = ref(false)
const loadError = ref<string | null>(null)
const loadingFiles = ref(false)

async function loadFiles() {
  loadError.value = null
  loadingFiles.value = true
  try {
    const status = await getContractFiles()
    files.value = status.files
  } catch (e) {
    loadError.value = e instanceof Error ? e.message : 'Failed to load contract files'
  } finally {
    loadingFiles.value = false
  }
}

function toggleFile(file: string) {
  const idx = selectedFiles.value.indexOf(file)
  if (idx >= 0) {
    selectedFiles.value.splice(idx, 1)
  } else {
    selectedFiles.value.push(file)
  }
}

function selectAll() {
  selectedFiles.value = [...files.value]
}

async function handleVerify() {
  if (!providerUrl.value) {
    toast.error('Provider URL is required')
    return
  }

  loading.value = true
  verified.value = false
  results.value = []

  try {
    results.value = await verifyContracts({
      files: selectedFiles.value,
      providerUrl: providerUrl.value,
      stateHandler: stateHandler.value || undefined,
    })
    verified.value = true
    const totalPassed = results.value.reduce((s, r) => s + r.passed, 0)
    const totalFailed = results.value.reduce((s, r) => s + r.failed, 0)
    if (totalFailed === 0) {
      toast.success(`All ${totalPassed} contracts passed`)
    } else {
      toast.error(`${totalFailed} contracts failed, ${totalPassed} passed`)
    }
  } catch (e) {
    toast.error(e instanceof Error ? e.message : 'Verification failed')
  } finally {
    loading.value = false
  }
}

const totalPassed = () => results.value.reduce((s, r) => s + r.passed, 0)
const totalFailed = () => results.value.reduce((s, r) => s + r.failed, 0)
const totalSkipped = () => results.value.reduce((s, r) => s + r.skipped, 0)

onMounted(loadFiles)
</script>

<template>
  <div class="h-full overflow-auto p-6">
      <div class="mb-4 flex items-center justify-between">
        <h1 class="text-lg font-semibold text-foreground">Contract Testing</h1>
        <button
          v-if="verified && totalFailed() === 0"
          class="flex items-center gap-1 rounded-md bg-success/10 px-2.5 py-1 text-xs font-medium text-success"
        >
          <CheckCircle :size="12" />
          All Passed
        </button>
      </div>

      <div class="space-y-4">
        <!-- Config -->
        <div class="rounded-lg border border-border bg-surface p-4 space-y-3">
          <h2 class="text-xs font-semibold uppercase tracking-wider text-muted-foreground/50">Provider Configuration</h2>
          <div class="grid grid-cols-2 gap-4">
            <div>
              <label class="mb-1 block text-xs text-muted-foreground">Provider URL</label>
              <input
                v-model="providerUrl"
                type="text"
                placeholder="http://localhost:3000"
                class="w-full rounded-md border border-border bg-input px-3 py-1.5 text-sm text-foreground focus:outline-none focus:ring-1 focus:ring-ring"
              />
            </div>
            <div>
              <label class="mb-1 block text-xs text-muted-foreground">State Handler (optional)</label>
              <input
                v-model="stateHandler"
                type="text"
                placeholder="./scripts/state-handler.sh"
                class="w-full rounded-md border border-border bg-input px-3 py-1.5 text-sm text-foreground focus:outline-none focus:ring-1 focus:ring-ring"
              />
            </div>
          </div>
        </div>

        <!-- File selection -->
        <div v-if="files.length > 0" class="rounded-lg border border-border bg-surface p-4 space-y-3">
          <div class="flex items-center justify-between">
            <h2 class="text-xs font-semibold uppercase tracking-wider text-muted-foreground/50">Contract Files</h2>
            <button class="text-xs text-accent hover:underline" @click="selectAll">Select All</button>
          </div>
          <div class="max-h-48 space-y-1 overflow-auto">
            <label
              v-for="file in files"
              :key="file"
              class="flex cursor-pointer items-center gap-2 rounded px-2 py-1 hover:bg-surface-hover"
            >
              <input
                type="checkbox"
                :checked="selectedFiles.includes(file)"
                class="accent-accent"
                @change="toggleFile(file)"
              />
              <span class="truncate font-mono text-sm text-foreground">{{ file }}</span>
            </label>
          </div>
        </div>

        <!-- Verify button -->
        <button
          class="flex items-center gap-1.5 rounded-md bg-accent px-4 py-1.5 text-sm font-medium text-accent-foreground hover:bg-accent/80 disabled:opacity-50"
          :disabled="loading || selectedFiles.length === 0 || !providerUrl.trim()"
          @click="handleVerify"
        >
          <Play :size="14" />
          {{ loading ? 'Verifying...' : 'Verify Contracts' }}
        </button>

        <!-- Loading files -->
        <LoadingSpinner v-if="loadingFiles" label="Loading contract files..." />

        <!-- Load error (always visible, not gated by v-else) -->
        <div v-if="loadError" class="flex items-start gap-2 rounded-lg border border-destructive/30 bg-destructive/5 p-3">
          <AlertCircle class="mt-0.5 h-4 w-4 shrink-0 text-destructive" />
          <span class="flex-1 text-xs text-destructive">{{ loadError }}</span>
          <button
            class="shrink-0 rounded-md border border-border px-2 py-0.5 text-xs text-muted-foreground transition-colors hover:bg-surface-hover hover:text-foreground"
            @click="loadFiles"
          >
            Retry
          </button>
        </div>

        <!-- Results -->
        <div v-if="verified" class="space-y-4">
          <div class="flex items-center gap-4">
            <div class="flex items-center gap-1.5 text-sm">
              <CheckCircle :size="14" class="text-success" />
              <span class="text-foreground">{{ totalPassed() }} passed</span>
            </div>
            <div class="flex items-center gap-1.5 text-sm">
              <XCircle :size="14" class="text-destructive" />
              <span class="text-foreground">{{ totalFailed() }} failed</span>
            </div>
            <div v-if="totalSkipped() > 0" class="flex items-center gap-1.5 text-sm">
              <AlertCircle :size="14" class="text-muted-foreground" />
              <span class="text-foreground">{{ totalSkipped() }} skipped</span>
            </div>
          </div>

          <div v-for="result in results" :key="result.file" class="rounded-lg border border-border bg-surface">
            <div class="flex items-center gap-2 border-b border-border px-4 py-2">
              <FileCheck :size="14" class="text-muted-foreground" />
              <span class="font-mono text-sm text-foreground">{{ result.file }}</span>
              <span class="ml-auto text-xs text-muted-foreground">{{ result.duration.toFixed(0) }}ms</span>
            </div>
            <div class="divide-y divide-border">
              <div
                v-for="(interaction, j) in result.results"
                :key="j"
                class="flex items-center gap-3 px-4 py-2"
              >
                <CheckCircle v-if="interaction.passed" :size="14" class="shrink-0 text-success" />
                <XCircle v-else :size="14" class="shrink-0 text-destructive" />
                <span class="text-sm text-foreground">{{ interaction.name || 'Unnamed' }}</span>
                <span v-if="interaction.state" class="rounded bg-background px-1.5 py-0.5 text-[10px] text-muted-foreground">
                  state: {{ interaction.state }}
                </span>
                <span class="ml-auto text-xs text-muted-foreground">{{ interaction.duration.toFixed(0) }}ms</span>
                <span v-if="interaction.error" class="max-w-xs truncate text-xs text-destructive">
                  {{ interaction.error }}
                </span>
              </div>
            </div>
          </div>
        </div>

        <EmptyState
          v-else-if="files.length === 0 && !loadError"
          :icon="FileCheck"
          title="No contract files found"
          description="Add .http or .hitspec files to your workspace to verify API contracts"
        />
      </div>
  </div>
</template>
