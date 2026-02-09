<script setup lang="ts">
import { computed } from 'vue'
import { statusCodeColor, statusCodeBgColor, STATUS_TEXT } from '@/lib/constants'

const props = defineProps<{ code: number }>()

const colorClass = computed(() => statusCodeColor(props.code))
const bgClass = computed(() => statusCodeBgColor(props.code))
const text = computed(() => STATUS_TEXT[props.code] ?? '')
</script>

<template>
  <span
    class="inline-flex items-center gap-1.5 rounded-md px-2 py-0.5 font-mono text-sm font-medium"
    :class="[colorClass, bgClass]"
  >
    {{ code }}
    <span v-if="text" class="text-xs opacity-80">{{ text }}</span>
  </span>
</template>
