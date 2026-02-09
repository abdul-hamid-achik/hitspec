<script setup lang="ts">
import { ref, toRef } from 'vue'
import { useCodeMirror } from '@/composables/useCodeMirror'

const props = defineProps<{
  content: string
  readonly?: boolean
}>()

const emit = defineEmits<{ change: [value: string] }>()

const container = ref<HTMLElement | null>(null)
const contentRef = toRef(props, 'content')

useCodeMirror(container, contentRef, {
  readonly: props.readonly,
  language: () => import('@codemirror/lang-json').then((m) => ({ default: m.json })),
  onChange: (value) => emit('change', value),
})
</script>

<template>
  <div ref="container" class="h-full overflow-hidden rounded-md border border-border" />
</template>
