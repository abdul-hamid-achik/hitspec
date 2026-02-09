<script setup lang="ts">
import AppShell from '@/components/layout/AppShell.vue'
import { ref } from 'vue'
import { importCurl } from '@/api/endpoints/import'

const activeTab = ref<'curl' | 'insomnia' | 'openapi'>('curl')
const curlCommand = ref('')
const result = ref<string | null>(null)
const error = ref<string | null>(null)

async function handleImportCurl() {
  error.value = null
  result.value = null
  try {
    const res = await importCurl(curlCommand.value)
    result.value = res.content
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  }
}
</script>

<template>
  <AppShell>
    <div class="mx-auto max-w-2xl p-6">
      <h1 class="mb-4 text-lg font-semibold text-foreground">Import</h1>
      <div class="mb-4 flex gap-2">
        <button
          v-for="tab in (['curl', 'insomnia', 'openapi'] as const)"
          :key="tab"
          class="rounded px-3 py-1.5 text-sm capitalize transition-colors"
          :class="activeTab === tab ? 'bg-accent text-accent-foreground' : 'bg-surface text-muted-foreground hover:text-foreground'"
          @click="activeTab = tab"
        >
          {{ tab === 'openapi' ? 'OpenAPI' : tab }}
        </button>
      </div>

      <div v-if="activeTab === 'curl'" class="space-y-4">
        <textarea
          v-model="curlCommand"
          placeholder="Paste your curl command here..."
          class="h-32 w-full rounded border border-border bg-background p-3 font-mono text-sm text-foreground placeholder:text-muted-foreground/50"
        />
        <button
          class="rounded bg-accent px-4 py-2 text-sm font-medium text-accent-foreground hover:bg-accent/80 disabled:opacity-50"
          :disabled="!curlCommand.trim()"
          @click="handleImportCurl"
        >
          Import
        </button>
      </div>

      <div v-if="error" class="mt-4 rounded border border-destructive/50 bg-destructive/10 p-3 text-sm text-destructive">
        {{ error }}
      </div>

      <div v-if="result" class="mt-4">
        <h3 class="mb-2 text-sm font-medium text-foreground">Generated .http file:</h3>
        <pre class="overflow-auto rounded border border-border bg-background p-4 font-mono text-xs text-foreground">{{ result }}</pre>
      </div>
    </div>
  </AppShell>
</template>
