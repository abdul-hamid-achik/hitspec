<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { X, Copy, Check } from 'lucide-vue-next'
import type { RequestDTO } from '@/types/api'
import { exporters, formatLabels, type ExportFormat } from '@/lib/exporters'

const props = defineProps<{
  modelValue: boolean
  request: RequestDTO
  variables: Record<string, string>
}>()

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
}>()

const activeFormat = ref<ExportFormat>('curl')
const copied = ref(false)

const formats: ExportFormat[] = ['curl', 'fetch', 'wget', 'python', 'httpie']

const code = computed(() => {
  const fn = exporters[activeFormat.value]
  return fn(props.request, props.variables)
})

watch(
  () => props.modelValue,
  (open) => {
    if (open) copied.value = false
  },
)

async function copyToClipboard() {
  await navigator.clipboard.writeText(code.value)
  copied.value = true
  setTimeout(() => (copied.value = false), 2000)
}

function close() {
  emit('update:modelValue', false)
}

function onOverlayClick(e: MouseEvent) {
  if (e.target === e.currentTarget) close()
}
</script>

<template>
  <Teleport to="body">
    <div
      v-if="modelValue"
      class="fixed inset-0 z-50 flex items-center justify-center bg-black/50"
      @click="onOverlayClick"
    >
      <div class="mx-4 w-full max-w-2xl rounded-lg border border-border bg-surface shadow-xl">
        <!-- Header -->
        <div class="flex items-center justify-between border-b border-border px-4 py-3">
          <h2 class="text-sm font-semibold text-foreground">Export Request</h2>
          <button
            class="rounded p-1 text-muted-foreground transition-colors hover:bg-surface-hover hover:text-foreground"
            @click="close"
          >
            <X class="h-4 w-4" />
          </button>
        </div>

        <!-- Format tabs -->
        <div class="flex gap-1 border-b border-border px-4 pt-3 pb-0">
          <button
            v-for="fmt in formats"
            :key="fmt"
            class="-mb-px rounded-t px-3 py-1.5 text-sm transition-colors"
            :class="
              activeFormat === fmt
                ? 'border border-b-surface border-border bg-surface text-foreground'
                : 'text-muted-foreground hover:text-foreground'
            "
            @click="activeFormat = fmt"
          >
            {{ formatLabels[fmt] }}
          </button>
        </div>

        <!-- Code block -->
        <div class="relative px-4 py-3">
          <pre
            class="max-h-80 overflow-auto rounded border border-border bg-background p-4 font-mono text-sm text-foreground"
          ><code>{{ code }}</code></pre>

          <!-- Copy button -->
          <button
            class="absolute top-5 right-6 rounded border border-border bg-surface px-2 py-1 text-xs text-muted-foreground transition-colors hover:bg-surface-hover hover:text-foreground"
            @click="copyToClipboard"
          >
            <span class="flex items-center gap-1">
              <Check v-if="copied" class="h-3 w-3 text-success" />
              <Copy v-else class="h-3 w-3" />
              {{ copied ? 'Copied' : 'Copy' }}
            </span>
          </button>
        </div>

        <!-- Footer -->
        <div class="flex justify-end border-t border-border px-4 py-3">
          <button
            class="rounded bg-accent px-4 py-1.5 text-sm font-medium text-accent-foreground transition-colors hover:bg-accent/80"
            @click="close"
          >
            Close
          </button>
        </div>
      </div>
    </div>
  </Teleport>
</template>
