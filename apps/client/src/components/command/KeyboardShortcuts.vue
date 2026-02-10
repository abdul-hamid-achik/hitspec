<script setup lang="ts">
import { DialogRoot, DialogPortal, DialogOverlay, DialogContent, DialogTitle } from 'reka-ui'
import { X } from 'lucide-vue-next'

defineProps<{ open: boolean }>()
const emit = defineEmits<{ 'update:open': [value: boolean] }>()

const isMac = navigator.platform.toLowerCase().includes('mac')
const mod = isMac ? '\u2318' : 'Ctrl'

const sections = [
  {
    title: 'General',
    shortcuts: [
      { keys: [mod, 'K'], label: 'Command palette' },
      { keys: [mod, 'Enter'], label: 'Run current file' },
      { keys: [mod, '?'], label: 'Show keyboard shortcuts' },
      { keys: ['Esc'], label: 'Dismiss errors / close dialogs' },
    ],
  },
  {
    title: 'Navigation',
    shortcuts: [
      { keys: [mod, '1'], label: 'Go to Workspace' },
      { keys: [mod, '2'], label: 'Go to Stress Testing' },
      { keys: [mod, '3'], label: 'Go to Mock Server' },
      { keys: [mod, '4'], label: 'Go to Contracts' },
      { keys: [mod, '5'], label: 'Go to Record' },
      { keys: [mod, '6'], label: 'Go to History' },
      { keys: [mod, '7'], label: 'Go to Import' },
      { keys: [mod, '8'], label: 'Go to Settings' },
    ],
  },
  {
    title: 'Sidebar',
    shortcuts: [
      { keys: [mod, 'B'], label: 'Toggle sidebar' },
    ],
  },
]
</script>

<template>
  <DialogRoot :open="open" @update:open="emit('update:open', $event)">
    <DialogPortal>
      <DialogOverlay class="fixed inset-0 z-50 bg-black/60 backdrop-blur-sm data-[state=open]:animate-in data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0" />
      <DialogContent class="fixed left-1/2 top-1/2 z-50 w-full max-w-md -translate-x-1/2 -translate-y-1/2 rounded-xl border border-border bg-surface shadow-2xl data-[state=open]:animate-in data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0 data-[state=closed]:zoom-out-95 data-[state=open]:zoom-in-95">
        <div class="flex items-center justify-between border-b border-border px-4 py-3">
          <DialogTitle class="text-sm font-semibold text-foreground">Keyboard Shortcuts</DialogTitle>
          <button
            aria-label="Close keyboard shortcuts"
            class="rounded p-1 text-muted-foreground transition-colors hover:bg-surface-hover hover:text-foreground"
            @click="emit('update:open', false)"
          >
            <X class="h-4 w-4" />
          </button>
        </div>
        <div class="max-h-96 overflow-y-auto p-4">
          <div v-for="section in sections" :key="section.title" class="mb-4 last:mb-0">
            <h3 class="mb-2 text-[10px] font-semibold uppercase tracking-wider text-muted-foreground/60">
              {{ section.title }}
            </h3>
            <div class="space-y-1.5">
              <div
                v-for="shortcut in section.shortcuts"
                :key="shortcut.label"
                class="flex items-center justify-between rounded px-2 py-1.5"
              >
                <span class="text-sm text-foreground">{{ shortcut.label }}</span>
                <div class="flex items-center gap-1">
                  <kbd
                    v-for="key in shortcut.keys"
                    :key="key"
                    class="inline-flex min-w-[22px] items-center justify-center rounded border border-border bg-background px-1.5 py-0.5 text-[11px] font-medium text-muted-foreground"
                  >
                    {{ key }}
                  </kbd>
                </div>
              </div>
            </div>
          </div>
        </div>
      </DialogContent>
    </DialogPortal>
  </DialogRoot>
</template>
