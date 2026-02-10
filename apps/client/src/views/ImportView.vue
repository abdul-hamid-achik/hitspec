<script setup lang="ts">
import AppShell from '@/components/layout/AppShell.vue'
import { ref } from 'vue'
import { importCurl } from '@/api/endpoints/import'
import { Copy, Check, AlertCircle } from 'lucide-vue-next'

const activeTab = ref<'curl' | 'insomnia' | 'openapi'>('curl')
const curlCommand = ref('')
const result = ref<string | null>(null)
const error = ref<string | null>(null)
const loading = ref(false)
const copied = ref(false)

async function handleImportCurl() {
  error.value = null
  result.value = null
  loading.value = true
  try {
    const res = await importCurl(curlCommand.value)
    result.value = res.content
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    loading.value = false
  }
}

async function copyResult() {
  if (!result.value) return
  await navigator.clipboard.writeText(result.value)
  copied.value = true
  setTimeout(() => (copied.value = false), 2000)
}
</script>

<template>
  <AppShell>
    <div class="mx-auto max-w-2xl p-6">
      <h1 class="mb-4 text-lg font-semibold text-foreground">Import</h1>

      <!-- Tab bar -->
      <div class="mb-4 flex gap-1">
        <button
          v-for="tab in (['curl', 'insomnia', 'openapi'] as const)"
          :key="tab"
          class="rounded-md px-3 py-1.5 text-xs font-medium capitalize transition-colors"
          :class="activeTab === tab
            ? 'bg-accent/15 text-accent'
            : 'text-muted-foreground hover:bg-surface-hover hover:text-foreground'"
          @click="activeTab = tab"
        >
          {{ tab === 'openapi' ? 'OpenAPI' : tab }}
        </button>
      </div>

      <!-- cURL tab -->
      <div v-if="activeTab === 'curl'" class="space-y-3">
        <div class="relative">
          <textarea
            v-model="curlCommand"
            placeholder="Paste your curl command here..."
            class="h-32 w-full resize-none rounded-lg border border-border bg-background p-3 font-mono text-xs text-foreground placeholder:text-muted-foreground/40 focus:outline-none focus:ring-1 focus:ring-ring"
          />
        </div>
        <button
          class="rounded-md bg-accent px-4 py-1.5 text-xs font-medium text-accent-foreground transition-colors hover:bg-accent/80 disabled:opacity-50"
          :disabled="!curlCommand.trim() || loading"
          @click="handleImportCurl"
        >
          {{ loading ? 'Importing...' : 'Import' }}
        </button>
      </div>

      <!-- Insomnia/OpenAPI placeholders -->
      <div v-else class="rounded-lg border border-border bg-surface p-6 text-center">
        <p class="text-sm text-muted-foreground/60">{{ activeTab === 'insomnia' ? 'Insomnia' : 'OpenAPI' }} import coming soon</p>
      </div>

      <!-- Error -->
      <div v-if="error" class="mt-4 flex items-start gap-2 rounded-lg border border-destructive/30 bg-destructive/5 p-3">
        <AlertCircle class="mt-0.5 h-4 w-4 shrink-0 text-destructive" />
        <span class="text-xs text-destructive">{{ error }}</span>
      </div>

      <!-- Result -->
      <div v-if="result" class="mt-4">
        <div class="mb-2 flex items-center justify-between">
          <h3 class="text-xs font-medium text-foreground">Generated .http file</h3>
          <button
            class="flex items-center gap-1 rounded-md border border-border px-2 py-0.5 text-[11px] text-muted-foreground transition-colors hover:bg-surface-hover hover:text-foreground"
            @click="copyResult"
          >
            <Check v-if="copied" class="h-3 w-3 text-success" />
            <Copy v-else class="h-3 w-3" />
            {{ copied ? 'Copied' : 'Copy' }}
          </button>
        </div>
        <pre class="max-h-64 overflow-auto rounded-lg border border-border bg-background p-4 font-mono text-xs leading-relaxed text-foreground/90">{{ result }}</pre>
      </div>
    </div>
  </AppShell>
</template>
