<script setup lang="ts">
import { ref, onErrorCaptured } from 'vue'
import { TriangleAlert, RotateCcw } from 'lucide-vue-next'

const error = ref<Error | null>(null)

onErrorCaptured((err: Error) => {
  error.value = err
  return false // prevent propagation
})

function reload() {
  window.location.reload()
}

function dismiss() {
  error.value = null
}

defineExpose({ error })
</script>

<template>
  <slot v-if="!error" />
  <div v-else class="flex h-full items-center justify-center bg-background p-8">
    <div class="flex max-w-md flex-col items-center gap-4 text-center">
      <div class="rounded-xl bg-destructive/10 p-4">
        <TriangleAlert class="h-10 w-10 text-destructive/70" />
      </div>
      <h2 class="text-lg font-semibold text-foreground">Something went wrong</h2>
      <p class="text-sm text-muted-foreground">
        {{ error.message || 'An unexpected error occurred.' }}
      </p>
      <div class="flex gap-2">
        <button
          class="flex items-center gap-1.5 rounded-md bg-accent px-4 py-2 text-sm font-medium text-accent-foreground transition-colors hover:bg-accent/80"
          @click="reload"
        >
          <RotateCcw class="h-4 w-4" />
          Reload
        </button>
        <button
          class="rounded-md border border-border px-4 py-2 text-sm text-muted-foreground transition-colors hover:bg-surface-hover hover:text-foreground"
          @click="dismiss"
        >
          Try Again
        </button>
      </div>
      <details class="w-full text-left">
        <summary class="cursor-pointer text-xs text-muted-foreground/50 hover:text-muted-foreground">
          Error details
        </summary>
        <pre class="mt-2 max-h-40 overflow-auto rounded-md bg-surface p-3 font-mono text-xs text-destructive/80">{{ error.stack }}</pre>
      </details>
    </div>
  </div>
</template>
