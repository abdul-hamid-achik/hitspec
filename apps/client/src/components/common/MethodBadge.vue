<script setup lang="ts">
import { computed } from 'vue'
import { METHOD_COLORS, METHOD_BG_COLORS, type HttpMethod } from '@/lib/constants'

const { method, size } = defineProps<{ method: string; size?: 'sm' | 'md' }>()
const colorClass = computed(() => METHOD_COLORS[method as HttpMethod] ?? 'text-muted-foreground')
const bgClass = computed(() => METHOD_BG_COLORS[method as HttpMethod] ?? 'bg-muted')
</script>

<template>
  <span
    :class="[
      'inline-flex items-center justify-center rounded font-mono font-bold uppercase',
      colorClass,
      bgClass,
      size === 'sm' ? 'px-1 py-0.5 text-[10px]' : 'px-1.5 py-0.5 text-xs',
    ]"
  >
    {{ method }}
  </span>
</template>
