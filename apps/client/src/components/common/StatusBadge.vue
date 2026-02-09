<script setup lang="ts">
import { statusCodeColor, statusCodeBgColor, STATUS_TEXT } from '@/lib/constants'

const props = defineProps<{ code: number }>()
</script>

<template>
  <span :class="['inline-flex items-center gap-1 rounded px-1.5 py-0.5 font-mono text-xs font-medium', statusCodeColor(props.code), statusCodeBgColor(props.code)]">
    {{ code }} {{ STATUS_TEXT[code] ?? '' }}
  </span>
</template>
