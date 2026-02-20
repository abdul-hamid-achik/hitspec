<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { importCurl, importInsomnia, importOpenAPI } from '@/api/endpoints/import'
import { useCollectionStore } from '@/stores/collection'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import { Copy, Check, AlertCircle, Save } from 'lucide-vue-next'
import { toast } from 'vue-sonner'

const router = useRouter()
const collection = useCollectionStore()

const activeTab = ref<'curl' | 'insomnia' | 'openapi' | 'postman'>('curl')
const curlCommand = ref('')
const insomniaJson = ref('')
const openapiSpec = ref('')
const openapiBaseUrl = ref('')
const postmanJson = ref('')
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

async function handleImportInsomnia() {
  error.value = null
  result.value = null
  loading.value = true
  try {
    const res = await importInsomnia(insomniaJson.value)
    result.value = res.content
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    loading.value = false
  }
}

async function handleImportOpenAPI() {
  error.value = null
  result.value = null
  loading.value = true
  try {
    const res = await importOpenAPI(openapiSpec.value, openapiBaseUrl.value || undefined)
    result.value = res.content
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    loading.value = false
  }
}

interface PostmanItem {
  name?: string
  request?: {
    method?: string
    url?: string | { raw?: string }
    header?: Array<{ key: string; value: string }>
    body?: { mode?: string; raw?: string }
  }
  item?: PostmanItem[]
}

function convertPostmanItems(items: PostmanItem[], indent = ''): string {
  const lines: string[] = []
  for (const item of items) {
    if (item.item) {
      // Folder — recurse
      if (item.name) lines.push(`# ${indent}${item.name}`)
      lines.push(convertPostmanItems(item.item, indent))
      continue
    }
    if (!item.request) continue
    const req = item.request
    const method = req.method || 'GET'
    const url = typeof req.url === 'string' ? req.url : req.url?.raw || ''

    lines.push(`### ${item.name || url}`)
    lines.push(`${method} ${url}`)

    if (req.header) {
      for (const h of req.header) {
        lines.push(`${h.key}: ${h.value}`)
      }
    }

    if (req.body?.raw) {
      lines.push('')
      lines.push(req.body.raw)
    }

    lines.push('')
  }
  return lines.join('\n')
}

function handleImportPostman() {
  error.value = null
  result.value = null
  try {
    const collection = JSON.parse(postmanJson.value)
    const items: PostmanItem[] = collection.item || []
    if (items.length === 0) {
      error.value = 'No requests found in collection'
      return
    }
    result.value = convertPostmanItems(items).trim()
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Invalid JSON'
  }
}

async function copyResult() {
  if (!result.value) return
  try {
    await navigator.clipboard.writeText(result.value)
    copied.value = true
    setTimeout(() => (copied.value = false), 2000)
  } catch {
    toast.error('Failed to copy to clipboard')
  }
}

const savingFile = ref(false)

async function saveAsHttpFile() {
  if (!result.value) return
  const name = prompt('File name (e.g. imported.http):', 'imported.http')
  if (!name) return
  const filename = name.endsWith('.http') || name.endsWith('.hitspec') ? name : `${name}.http`
  savingFile.value = true
  try {
    await collection.createNewFile(filename, result.value)
    toast.success(`Saved as ${filename}`)
    router.push('/')
  } catch (e) {
    toast.error(e instanceof Error ? e.message : 'Failed to save file')
  } finally {
    savingFile.value = false
  }
}
</script>

<template>
  <div class="h-full overflow-auto"><div class="mx-auto max-w-2xl p-6">
      <h1 class="mb-4 text-lg font-semibold text-foreground">Import</h1>

      <!-- Tab bar -->
      <div class="mb-4 flex gap-1">
        <button
          v-for="tab in (['curl', 'insomnia', 'openapi', 'postman'] as const)"
          :key="tab"
          class="rounded-md px-3 py-1.5 text-xs font-medium capitalize transition-colors"
          :class="activeTab === tab
            ? 'bg-accent/15 text-accent'
            : 'text-muted-foreground hover:bg-surface-hover hover:text-foreground'"
          @click="activeTab = tab"
        >
          {{ tab === 'openapi' ? 'OpenAPI' : tab === 'postman' ? 'Postman' : tab }}
        </button>
      </div>

      <!-- cURL tab -->
      <div v-if="activeTab === 'curl'" class="space-y-3">
        <div class="relative">
          <textarea
            v-model="curlCommand"
            placeholder="Paste your curl command here..."
            class="h-32 w-full resize-none rounded-lg border border-border bg-background p-3 font-mono text-xs text-foreground placeholder:text-muted-foreground/60 focus:outline-none focus:ring-1 focus:ring-ring"
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

      <!-- Insomnia tab -->
      <div v-else-if="activeTab === 'insomnia'" class="space-y-3">
        <div class="relative">
          <textarea
            v-model="insomniaJson"
            placeholder='Paste your Insomnia export JSON here... {"_type":"export","resources":[...]}'
            class="h-32 w-full resize-none rounded-lg border border-border bg-background p-3 font-mono text-xs text-foreground placeholder:text-muted-foreground/60 focus:outline-none focus:ring-1 focus:ring-ring"
          />
        </div>
        <button
          class="rounded-md bg-accent px-4 py-1.5 text-xs font-medium text-accent-foreground transition-colors hover:bg-accent/80 disabled:opacity-50"
          :disabled="!insomniaJson.trim() || loading"
          @click="handleImportInsomnia"
        >
          {{ loading ? 'Importing...' : 'Import' }}
        </button>
      </div>

      <!-- OpenAPI tab -->
      <div v-else-if="activeTab === 'openapi'" class="space-y-3">
        <div>
          <label class="mb-1 block text-xs text-muted-foreground">OpenAPI spec file path</label>
          <input
            v-model="openapiSpec"
            type="text"
            placeholder="./openapi.yaml or ./swagger.json"
            class="w-full rounded-md border border-border bg-background px-3 py-1.5 font-mono text-xs text-foreground placeholder:text-muted-foreground/60 focus:outline-none focus:ring-1 focus:ring-ring"
          />
        </div>
        <div>
          <label class="mb-1 block text-xs text-muted-foreground">Base URL (optional)</label>
          <input
            v-model="openapiBaseUrl"
            type="text"
            placeholder="http://localhost:3000"
            class="w-full rounded-md border border-border bg-background px-3 py-1.5 font-mono text-xs text-foreground placeholder:text-muted-foreground/60 focus:outline-none focus:ring-1 focus:ring-ring"
          />
        </div>
        <button
          class="rounded-md bg-accent px-4 py-1.5 text-xs font-medium text-accent-foreground transition-colors hover:bg-accent/80 disabled:opacity-50"
          :disabled="!openapiSpec.trim() || loading"
          @click="handleImportOpenAPI"
        >
          {{ loading ? 'Importing...' : 'Import' }}
        </button>
      </div>

      <!-- Postman tab -->
      <div v-else-if="activeTab === 'postman'" class="space-y-3">
        <div class="relative">
          <textarea
            v-model="postmanJson"
            placeholder='Paste your Postman Collection v2.1 JSON here... {"info":{"name":"..."},"item":[...]}'
            class="h-32 w-full resize-none rounded-lg border border-border bg-background p-3 font-mono text-xs text-foreground placeholder:text-muted-foreground/60 focus:outline-none focus:ring-1 focus:ring-ring"
          />
        </div>
        <p class="text-[10px] text-muted-foreground/50">
          Export from Postman: Collection &rarr; Export &rarr; Collection v2.1
        </p>
        <button
          class="rounded-md bg-accent px-4 py-1.5 text-xs font-medium text-accent-foreground transition-colors hover:bg-accent/80 disabled:opacity-50"
          :disabled="!postmanJson.trim()"
          @click="handleImportPostman"
        >
          Import
        </button>
      </div>

      <!-- Loading spinner -->
      <div v-if="loading" class="mt-4">
        <LoadingSpinner label="Importing..." />
      </div>

      <!-- Error -->
      <div v-if="error" class="mt-4 flex items-start gap-2 rounded-lg border border-destructive/30 bg-destructive/5 p-3">
        <AlertCircle class="mt-0.5 h-4 w-4 shrink-0 text-destructive" />
        <span class="flex-1 text-xs text-destructive">{{ error }}</span>
        <button
          class="shrink-0 rounded-md border border-border px-2 py-0.5 text-xs text-muted-foreground transition-colors hover:bg-surface-hover hover:text-foreground"
          @click="error = null"
        >
          Dismiss
        </button>
      </div>

      <!-- Result -->
      <div v-if="result" class="mt-4">
        <div class="mb-2 flex items-center justify-between">
          <h3 class="text-xs font-medium text-foreground">Generated .http file</h3>
          <div class="flex items-center gap-1.5">
            <button
              class="flex items-center gap-1 rounded-md border border-border px-2 py-0.5 text-[11px] text-muted-foreground transition-colors hover:bg-surface-hover hover:text-foreground"
              @click="copyResult"
            >
              <Check v-if="copied" class="h-3 w-3 text-success" />
              <Copy v-else class="h-3 w-3" />
              {{ copied ? 'Copied' : 'Copy' }}
            </button>
            <button
              class="flex items-center gap-1 rounded-md bg-accent px-2 py-0.5 text-[11px] font-medium text-accent-foreground transition-colors hover:bg-accent/80 disabled:opacity-50"
              :disabled="savingFile"
              @click="saveAsHttpFile"
            >
              <Save class="h-3 w-3" />
              {{ savingFile ? 'Saving...' : 'Save as .http' }}
            </button>
          </div>
        </div>
        <pre class="max-h-64 overflow-auto rounded-lg border border-border bg-background p-4 font-mono text-xs leading-relaxed text-foreground/90">{{ result }}</pre>
      </div>
  </div></div>
</template>
