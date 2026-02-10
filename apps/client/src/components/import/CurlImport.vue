<script setup lang="ts">
import { ref } from 'vue'
import { importCurl } from '@/api/endpoints/import'
import { toast } from 'vue-sonner'

const curlCommand = ref('')
const result = ref<string | null>(null)

async function handleImport() {
  if (!curlCommand.value.trim()) {
    toast.error('Please enter a curl command')
    return
  }
  try {
    const parsed = await importCurl(curlCommand.value)
    result.value = parsed.content
    toast.success('Imported successfully')
  } catch (e) {
    toast.error(e instanceof Error ? e.message : 'Import failed')
  }
}
</script>

<template>
  <div class="space-y-4">
    <div>
      <label class="mb-1 block text-sm text-muted-foreground">Paste your cURL command</label>
      <textarea
        v-model="curlCommand"
        rows="6"
        placeholder="curl -X POST https://api.example.com/users -H 'Content-Type: application/json' -d '{...}'"
        class="w-full rounded-md border border-border bg-input p-3 font-mono text-sm text-foreground placeholder:text-nord-3 focus:outline-none focus:ring-1 focus:ring-ring"
      />
    </div>
    <button
      class="rounded-md bg-accent px-4 py-1.5 text-sm font-medium text-accent-foreground hover:bg-accent/80"
      @click="handleImport"
    >
      Import
    </button>
    <div v-if="result" class="rounded-md border border-success/30 bg-success/5 p-3">
      <pre class="font-mono text-sm text-foreground">{{ result }}</pre>
    </div>
  </div>
</template>
