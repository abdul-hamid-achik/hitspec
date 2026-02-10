<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { DialogRoot, DialogPortal, DialogOverlay, DialogContent, DialogTitle } from 'reka-ui'
import { X, Copy, Check, Download } from 'lucide-vue-next'
import type { RequestDTO } from '@/types/api'
import { exporters, formatLabels, type ExportFormat } from '@/lib/exporters'
import { toast } from 'vue-sonner'

const { modelValue, request, variables } = defineProps<{
  modelValue: boolean
  request: RequestDTO
  variables: Record<string, unknown>
}>()

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
}>()

const activeFormat = ref<ExportFormat>('curl')
const copied = ref(false)

const formats: ExportFormat[] = ['curl', 'fetch', 'wget', 'python', 'httpie', 'go', 'ruby']

const stringVars = computed(() => {
  const result: Record<string, string> = {}
  for (const [k, v] of Object.entries(variables)) {
    result[k] = String(v ?? '')
  }
  return result
})

const code = computed(() => {
  const fn = exporters[activeFormat.value]
  return fn(request, stringVars.value)
})

watch(
  () => modelValue,
  (open) => {
    if (open) copied.value = false
  },
)

async function copyToClipboard() {
  try {
    await navigator.clipboard.writeText(code.value)
    copied.value = true
    setTimeout(() => (copied.value = false), 2000)
  } catch {
    toast.error('Failed to copy to clipboard')
  }
}

const formatExtensions: Record<ExportFormat, string> = {
  curl: '.sh',
  fetch: '.js',
  wget: '.sh',
  python: '.py',
  httpie: '.sh',
  go: '.go',
  ruby: '.rb',
}

function download() {
  const blob = new Blob([code.value], { type: 'text/plain' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  const name = request.name || 'request'
  a.download = `${name}${formatExtensions[activeFormat.value]}`
  a.click()
  URL.revokeObjectURL(url)
}

function close() {
  emit('update:modelValue', false)
}
</script>

<template>
  <DialogRoot :open="modelValue" @update:open="emit('update:modelValue', $event)">
    <DialogPortal>
      <DialogOverlay class="fixed inset-0 z-50 bg-black/60 backdrop-blur-sm data-[state=open]:animate-in data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0" />
      <DialogContent class="fixed left-1/2 top-1/2 z-50 w-full max-w-2xl -translate-x-1/2 -translate-y-1/2 rounded-xl border border-border bg-surface shadow-2xl data-[state=open]:animate-in data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0 data-[state=closed]:zoom-out-95 data-[state=open]:zoom-in-95">
        <!-- Header -->
        <div class="flex items-center justify-between border-b border-border px-4 py-3">
          <DialogTitle class="text-sm font-semibold text-foreground">Export Request</DialogTitle>
          <button
            aria-label="Close export dialog"
            class="rounded p-1 text-muted-foreground transition-colors hover:bg-surface-hover hover:text-foreground"
            @click="close"
          >
            <X class="h-4 w-4" />
          </button>
        </div>

        <!-- Format tabs -->
        <div class="flex gap-1 border-b border-border px-4 py-2">
          <button
            v-for="fmt in formats"
            :key="fmt"
            class="rounded-md px-3 py-1 text-xs font-medium transition-colors"
            :class="
              activeFormat === fmt
                ? 'bg-accent/15 text-accent'
                : 'text-muted-foreground hover:bg-surface-hover hover:text-foreground'
            "
            @click="activeFormat = fmt"
          >
            {{ formatLabels[fmt] }}
          </button>
        </div>

        <!-- Code block -->
        <div class="relative px-4 py-3">
          <pre
            class="max-h-80 overflow-auto rounded-lg border border-border bg-background p-4 font-mono text-xs leading-relaxed text-foreground"
          ><code>{{ code }}</code></pre>

          <!-- Copy button -->
          <button
            class="absolute top-5 right-6 flex items-center gap-1 rounded-md border border-border bg-surface px-2 py-1 text-xs text-muted-foreground transition-colors hover:bg-surface-hover hover:text-foreground"
            @click="copyToClipboard"
          >
            <Check v-if="copied" class="h-3 w-3 text-success" />
            <Copy v-else class="h-3 w-3" />
            {{ copied ? 'Copied!' : 'Copy' }}
          </button>
        </div>

        <!-- Footer -->
        <div class="flex justify-end gap-2 border-t border-border px-4 py-3">
          <button
            class="flex items-center gap-1.5 rounded-md bg-accent px-3 py-1.5 text-xs font-medium text-accent-foreground transition-colors hover:bg-accent/80"
            @click="download"
          >
            <Download class="h-3 w-3" />
            Download
          </button>
          <button
            class="rounded-md border border-border px-3 py-1.5 text-xs text-muted-foreground transition-colors hover:bg-surface-hover hover:text-foreground"
            @click="close"
          >
            Close
          </button>
        </div>
      </DialogContent>
    </DialogPortal>
  </DialogRoot>
</template>
