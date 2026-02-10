<script setup lang="ts">
import { computed, ref } from 'vue'
import { Copy, Check, AlignLeft } from 'lucide-vue-next'

const props = defineProps<{ body?: string }>()

const copied = ref(false)

const formatted = computed(() => {
  if (!props.body) return ''
  try {
    return JSON.stringify(JSON.parse(props.body), null, 2)
  } catch {
    return props.body
  }
})

const isJson = computed(() => {
  if (!props.body) return false
  try {
    JSON.parse(props.body)
    return true
  } catch {
    return false
  }
})

const lineCount = computed(() => formatted.value.split('\n').length)

async function copyBody() {
  await navigator.clipboard.writeText(formatted.value)
  copied.value = true
  setTimeout(() => (copied.value = false), 2000)
}
</script>

<template>
  <div v-if="formatted" class="relative h-full">
    <div class="absolute right-3 top-3 z-10 flex items-center gap-2">
      <span v-if="isJson" class="rounded bg-accent/10 px-1.5 py-0.5 text-[10px] font-medium text-accent">JSON</span>
      <span class="text-[10px] text-muted-foreground/40">{{ lineCount }} lines</span>
      <button
        class="flex items-center gap-1 rounded-md border border-border bg-surface px-2 py-0.5 text-[11px] text-muted-foreground transition-colors hover:bg-surface-hover hover:text-foreground"
        @click="copyBody"
      >
        <Check v-if="copied" class="h-3 w-3 text-success" />
        <Copy v-else class="h-3 w-3" />
        {{ copied ? 'Copied' : 'Copy' }}
      </button>
    </div>
    <pre class="h-full overflow-auto p-4 font-mono text-xs leading-relaxed text-foreground/90">{{ formatted }}</pre>
  </div>
  <div v-else class="flex flex-col items-center gap-2 py-8 text-center">
    <AlignLeft class="h-8 w-8 text-muted-foreground/30" />
    <span class="text-xs text-muted-foreground/60">No response body</span>
  </div>
</template>
