<script setup lang="ts">
import { METHOD_COLORS, METHOD_BG_COLORS, type HttpMethod } from '@/lib/constants'

const props = defineProps<{ method: string; size?: 'sm' | 'md' }>()
const colorClass = METHOD_COLORS[props.method as HttpMethod] ?? 'text-muted-foreground'
const bgClass = METHOD_BG_COLORS[props.method as HttpMethod] ?? 'bg-muted'
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
