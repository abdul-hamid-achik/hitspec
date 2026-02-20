<script setup lang="ts">
import { ref, computed } from 'vue'

const { direction = 'horizontal', minLeft = 200, minRight = 200 } = defineProps<{
  direction?: 'horizontal' | 'vertical'
  minLeft?: number
  minRight?: number
}>()

const ratio = defineModel<number>('ratio', { default: 0.5 })

const containerRef = ref<HTMLElement | null>(null)
const dragging = ref(false)

const leftStyle = computed(() => ({
  flexBasis: `${ratio.value * 100}%`,
  flexGrow: 0,
  flexShrink: 0,
  minWidth: `${minLeft}px`,
  overflow: 'hidden',
}))

const rightStyle = computed(() => ({
  flex: 1,
  minWidth: `${minRight}px`,
  overflow: 'hidden',
}))

function onPointerDown(e: PointerEvent) {
  e.preventDefault()
  dragging.value = true
  const target = e.currentTarget as HTMLElement
  target.setPointerCapture(e.pointerId)
}

function onPointerMove(e: PointerEvent) {
  if (!dragging.value || !containerRef.value) return
  const rect = containerRef.value.getBoundingClientRect()
  const totalSize = direction === 'horizontal' ? rect.width : rect.height
  const pos = direction === 'horizontal'
    ? e.clientX - rect.left
    : e.clientY - rect.top
  const newRatio = pos / totalSize
  // Clamp to min boundaries
  const minRatio = minLeft / totalSize
  const maxRatio = 1 - minRight / totalSize
  ratio.value = Math.max(minRatio, Math.min(maxRatio, newRatio))
}

function onPointerUp() {
  dragging.value = false
}
</script>

<template>
  <div
    ref="containerRef"
    class="flex h-full"
    :class="direction === 'vertical' ? 'flex-col' : 'flex-row'"
  >
    <div :style="leftStyle">
      <slot name="left" />
    </div>
    <div
      class="shrink-0 select-none"
      :class="[
        direction === 'horizontal'
          ? 'w-1.5 cursor-col-resize hover:bg-accent/20'
          : 'h-1.5 cursor-row-resize hover:bg-accent/20',
        dragging ? 'bg-accent/30' : 'bg-border',
      ]"
      @pointerdown="onPointerDown"
      @pointermove="onPointerMove"
      @pointerup="onPointerUp"
      @lostpointercapture="onPointerUp"
    />
    <div :style="rightStyle">
      <slot name="right" />
    </div>
  </div>
</template>
