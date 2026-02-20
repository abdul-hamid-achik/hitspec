<script setup lang="ts">
import { DialogRoot, DialogPortal, DialogOverlay, DialogContent, DialogTitle, DialogDescription } from 'reka-ui'
import { useConfirm } from '@/composables/useConfirm'

const { visible, options, handleConfirm, handleCancel } = useConfirm()
</script>

<template>
  <DialogRoot :open="visible" @update:open="(v) => { if (!v) handleCancel() }">
    <DialogPortal>
      <DialogOverlay class="fixed inset-0 z-50 bg-black/60 backdrop-blur-sm data-[state=open]:animate-in data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0" />
      <DialogContent class="fixed left-1/2 top-1/2 z-50 w-full max-w-sm -translate-x-1/2 -translate-y-1/2 rounded-xl border border-border bg-surface shadow-2xl data-[state=open]:animate-in data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0 data-[state=closed]:zoom-out-95 data-[state=open]:zoom-in-95">
        <div class="p-5">
          <DialogTitle class="text-sm font-semibold text-foreground">
            {{ options.title }}
          </DialogTitle>
          <DialogDescription class="mt-2 text-xs text-muted-foreground">
            {{ options.message }}
          </DialogDescription>
        </div>
        <div class="flex justify-end gap-2 border-t border-border px-5 py-3">
          <button
            class="rounded-md border border-border px-3 py-1.5 text-xs text-muted-foreground transition-colors hover:bg-surface-hover hover:text-foreground"
            @click="handleCancel"
          >
            {{ options.cancelLabel ?? 'Cancel' }}
          </button>
          <button
            class="rounded-md px-3 py-1.5 text-xs font-medium transition-colors"
            :class="options.variant === 'destructive'
              ? 'bg-destructive text-destructive-foreground hover:bg-destructive/90'
              : 'bg-accent text-accent-foreground hover:bg-accent/90'"
            @click="handleConfirm"
          >
            {{ options.confirmLabel ?? 'Confirm' }}
          </button>
        </div>
      </DialogContent>
    </DialogPortal>
  </DialogRoot>
</template>
