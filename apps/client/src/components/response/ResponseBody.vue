<script setup lang="ts">
import { computed, ref } from 'vue'
import { Copy, Check, AlignLeft } from 'lucide-vue-next'
import { toast } from 'vue-sonner'
import { useCodeMirror } from '@/composables/useCodeMirror'

const { body, error } = defineProps<{ body?: string; error?: string }>()

const copied = ref(false)
const bodyEditorRef = ref<HTMLElement | null>(null)

const parsed = computed(() => {
  if (!body) return { isJson: false, formatted: '' }
  try {
    const obj = JSON.parse(body)
    return { isJson: true, formatted: JSON.stringify(obj, null, 2) }
  } catch {
    return { isJson: false, formatted: body }
  }
})

const isJson = computed(() => parsed.value.isJson)
const formatted = computed(() => parsed.value.formatted)

const lineCount = computed(() => {
  const text = formatted.value
  if (!text) return 0
  let count = 1
  for (let i = 0; i < text.length; i++) {
    if (text.charCodeAt(i) === 10) count++
  }
  return count
})

useCodeMirror(bodyEditorRef, formatted, {
  readonly: true,
  language: () => import('@codemirror/lang-json').then(m => ({ default: m.json })),
})

async function copyBody() {
  try {
    await navigator.clipboard.writeText(formatted.value)
    copied.value = true
    setTimeout(() => (copied.value = false), 2000)
  } catch {
    toast.error('Failed to copy to clipboard')
  }
}
</script>

<template>
  <div v-if="formatted" class="relative flex h-full flex-col">
    <div class="flex items-center justify-end gap-2 border-b border-border px-4 py-1.5">
      <span v-if="isJson" class="rounded bg-accent/10 px-1.5 py-0.5 text-[10px] font-medium text-accent">JSON</span>
      <span class="text-[10px] text-muted-foreground/60">{{ lineCount }} lines</span>
      <button
        class="flex items-center gap-1 rounded-md border border-border bg-surface px-2 py-0.5 text-[11px] text-muted-foreground transition-colors hover:bg-surface-hover hover:text-foreground"
        @click="copyBody"
      >
        <Check v-if="copied" class="h-3 w-3 text-success" />
        <Copy v-else class="h-3 w-3" />
        {{ copied ? 'Copied' : 'Copy' }}
      </button>
    </div>
    <div ref="bodyEditorRef" class="flex-1 overflow-auto" />
  </div>
  <div v-else-if="error" class="p-4">
    <div class="rounded-lg border border-destructive/20 bg-destructive/5 p-4">
      <h4 class="mb-1 text-xs font-medium text-destructive">Request Error</h4>
      <pre class="whitespace-pre-wrap font-mono text-xs text-destructive/80">{{ error }}</pre>
    </div>
  </div>
  <div v-else class="flex flex-col items-center gap-2 py-8 text-center">
    <AlignLeft class="h-8 w-8 text-muted-foreground/50" />
    <span class="text-xs text-muted-foreground/60">No response body</span>
  </div>
</template>
