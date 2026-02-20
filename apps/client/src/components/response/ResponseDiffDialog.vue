<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { DialogRoot, DialogPortal, DialogOverlay, DialogContent, DialogTitle } from 'reka-ui'
import { X, Copy, Check, Columns2, AlignJustify, Loader2 } from 'lucide-vue-next'
import { formatUnifiedDiff, type DiffLine } from '@/lib/diff'
import { useDiffWorker } from '@/composables/useDiffWorker'
import { toast } from 'vue-sonner'

export interface DiffSource {
  body: string
  label: string
}

const { modelValue, left, right } = defineProps<{
  modelValue: boolean
  left: DiffSource
  right: DiffSource
}>()

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
}>()

const viewMode = ref<'unified' | 'split'>('unified')
const copied = ref(false)

const { compute: computeDiffAsync, computing } = useDiffWorker()
const diffLines = ref<DiffLine[]>([])

const stats = computed(() => {
  let added = 0
  let removed = 0
  for (const l of diffLines.value) {
    if (l.type === 'add') added++
    else if (l.type === 'remove') removed++
  }
  return { added, removed }
})

// Split view: pair up removes/adds into left/right columns
const splitPairs = computed(() => {
  const pairs: Array<{ left: DiffLine | null; right: DiffLine | null }> = []
  const lines = diffLines.value
  let i = 0
  while (i < lines.length) {
    const line = lines[i]
    if (line.type === 'equal') {
      pairs.push({ left: line, right: line })
      i++
    } else if (line.type === 'remove') {
      const removes: DiffLine[] = []
      while (i < lines.length && lines[i].type === 'remove') {
        removes.push(lines[i])
        i++
      }
      const adds: DiffLine[] = []
      while (i < lines.length && lines[i].type === 'add') {
        adds.push(lines[i])
        i++
      }
      const max = Math.max(removes.length, adds.length)
      for (let j = 0; j < max; j++) {
        pairs.push({
          left: j < removes.length ? removes[j] : null,
          right: j < adds.length ? adds[j] : null,
        })
      }
    } else {
      pairs.push({ left: null, right: line })
      i++
    }
  }
  return pairs
})

watch(
  () => modelValue,
  async (open) => {
    if (open) {
      copied.value = false
      diffLines.value = []
      diffLines.value = await computeDiffAsync(left.body, right.body)
    }
  },
)

async function copyDiff() {
  try {
    const text = formatUnifiedDiff(diffLines.value, left.label, right.label)
    await navigator.clipboard.writeText(text)
    copied.value = true
    setTimeout(() => (copied.value = false), 2000)
  } catch {
    toast.error('Failed to copy to clipboard')
  }
}

function close() {
  emit('update:modelValue', false)
}

function lineClass(type: DiffLine['type']): string {
  if (type === 'add') return 'bg-success/10 text-success'
  if (type === 'remove') return 'bg-destructive/10 text-destructive'
  return 'text-foreground/80'
}

function gutterClass(type: DiffLine['type']): string {
  if (type === 'add') return 'bg-success/15 text-success/60'
  if (type === 'remove') return 'bg-destructive/15 text-destructive/60'
  return 'text-muted-foreground/50'
}

function prefix(type: DiffLine['type']): string {
  if (type === 'add') return '+'
  if (type === 'remove') return '-'
  return ' '
}
</script>

<template>
  <DialogRoot :open="modelValue" @update:open="emit('update:modelValue', $event)">
    <DialogPortal>
      <DialogOverlay class="fixed inset-0 z-50 bg-black/60 backdrop-blur-sm data-[state=open]:animate-in data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0" />
      <DialogContent class="fixed left-1/2 top-1/2 z-50 w-full max-w-4xl -translate-x-1/2 -translate-y-1/2 rounded-xl border border-border bg-surface shadow-2xl data-[state=open]:animate-in data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0 data-[state=closed]:zoom-out-95 data-[state=open]:zoom-in-95">
        <!-- Header -->
        <div class="flex items-center justify-between border-b border-border px-4 py-3">
          <DialogTitle class="text-sm font-semibold text-foreground">Response Diff</DialogTitle>
          <div class="flex items-center gap-2">
            <span v-if="stats.added > 0" class="text-[11px] font-medium text-success">+{{ stats.added }}</span>
            <span v-if="stats.removed > 0" class="text-[11px] font-medium text-destructive">-{{ stats.removed }}</span>
            <button
              aria-label="Close diff dialog"
              class="rounded p-1 text-muted-foreground transition-colors hover:bg-surface-hover hover:text-foreground"
              @click="close"
            >
              <X class="h-4 w-4" />
            </button>
          </div>
        </div>

        <!-- View mode tabs + actions -->
        <div class="flex items-center justify-between border-b border-border px-4 py-2">
          <div class="flex gap-1">
            <button
              class="flex items-center gap-1.5 rounded-md px-3 py-1 text-xs font-medium transition-colors"
              :class="viewMode === 'unified' ? 'bg-accent/15 text-accent' : 'text-muted-foreground hover:bg-surface-hover hover:text-foreground'"
              @click="viewMode = 'unified'"
            >
              <AlignJustify class="h-3 w-3" />
              Unified
            </button>
            <button
              class="flex items-center gap-1.5 rounded-md px-3 py-1 text-xs font-medium transition-colors"
              :class="viewMode === 'split' ? 'bg-accent/15 text-accent' : 'text-muted-foreground hover:bg-surface-hover hover:text-foreground'"
              @click="viewMode = 'split'"
            >
              <Columns2 class="h-3 w-3" />
              Split
            </button>
          </div>
          <button
            :disabled="computing"
            class="flex items-center gap-1 rounded-md border border-border bg-surface px-2 py-1 text-xs text-muted-foreground transition-colors hover:bg-surface-hover hover:text-foreground disabled:opacity-50"
            @click="copyDiff"
          >
            <Check v-if="copied" class="h-3 w-3 text-success" />
            <Copy v-else class="h-3 w-3" />
            {{ copied ? 'Copied!' : 'Copy diff' }}
          </button>
        </div>

        <!-- Labels -->
        <div v-if="viewMode === 'split'" class="flex border-b border-border text-[11px] font-medium">
          <div class="flex-1 px-4 py-1.5 text-destructive/70">{{ left.label }}</div>
          <div class="flex-1 border-l border-border px-4 py-1.5 text-success/70">{{ right.label }}</div>
        </div>
        <div v-else class="flex items-center gap-3 border-b border-border px-4 py-1.5 text-[11px] font-medium">
          <span class="text-destructive/70">{{ left.label }}</span>
          <span class="text-muted-foreground/50">&rarr;</span>
          <span class="text-success/70">{{ right.label }}</span>
        </div>

        <!-- Diff content -->
        <div class="max-h-[60vh] overflow-auto">
          <!-- Computing spinner -->
          <div v-if="computing" class="flex items-center justify-center gap-2 p-8">
            <Loader2 class="h-5 w-5 animate-spin text-accent" />
            <span class="text-sm text-muted-foreground">Computing diff...</span>
          </div>

          <!-- Unified view -->
          <div v-else-if="viewMode === 'unified'" class="font-mono text-xs leading-relaxed">
            <div v-if="diffLines.length === 0" class="p-8 text-center text-sm text-muted-foreground">
              Responses are identical
            </div>
            <div
              v-for="(line, i) in diffLines"
              :key="i"
              class="flex"
              :class="lineClass(line.type)"
            >
              <span class="w-10 shrink-0 select-none px-2 py-px text-right" :class="gutterClass(line.type)">
                {{ line.oldLineNum ?? '' }}
              </span>
              <span class="w-10 shrink-0 select-none px-2 py-px text-right" :class="gutterClass(line.type)">
                {{ line.newLineNum ?? '' }}
              </span>
              <span class="shrink-0 select-none px-1 py-px" :class="gutterClass(line.type)">{{ prefix(line.type) }}</span>
              <span class="flex-1 whitespace-pre-wrap break-all px-2 py-px">{{ line.content }}</span>
            </div>
          </div>

          <!-- Split view -->
          <div v-else class="font-mono text-xs leading-relaxed">
            <div v-if="splitPairs.length === 0" class="p-8 text-center text-sm text-muted-foreground">
              Responses are identical
            </div>
            <div
              v-for="(pair, i) in splitPairs"
              :key="i"
              class="flex"
            >
              <!-- Left side -->
              <div
                class="flex flex-1 min-w-0"
                :class="pair.left ? lineClass(pair.left.type === 'equal' ? 'equal' : 'remove') : ''"
              >
                <span
                  class="w-10 shrink-0 select-none px-2 py-px text-right"
                  :class="pair.left ? gutterClass(pair.left.type === 'equal' ? 'equal' : 'remove') : 'text-muted-foreground/20'"
                >
                  {{ pair.left?.oldLineNum ?? '' }}
                </span>
                <span class="flex-1 whitespace-pre-wrap break-all px-2 py-px">{{ pair.left?.content ?? '' }}</span>
              </div>
              <!-- Right side -->
              <div
                class="flex flex-1 min-w-0 border-l border-border/50"
                :class="pair.right ? lineClass(pair.right.type === 'equal' ? 'equal' : 'add') : ''"
              >
                <span
                  class="w-10 shrink-0 select-none px-2 py-px text-right"
                  :class="pair.right ? gutterClass(pair.right.type === 'equal' ? 'equal' : 'add') : 'text-muted-foreground/20'"
                >
                  {{ pair.right?.newLineNum ?? '' }}
                </span>
                <span class="flex-1 whitespace-pre-wrap break-all px-2 py-px">{{ pair.right?.content ?? '' }}</span>
              </div>
            </div>
          </div>
        </div>

        <!-- Footer -->
        <div class="flex justify-end border-t border-border px-4 py-3">
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
