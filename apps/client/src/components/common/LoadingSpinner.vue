<script setup lang="ts">
defineProps<{
  size?: 'sm' | 'md' | 'lg'
  label?: string
}>()
</script>

<template>
  <div class="flex flex-col items-center justify-center gap-2 p-4">
    <div
      :class="[
        'animate-spin rounded-full border-2 border-accent/30 border-t-accent',
        size === 'sm' ? 'h-4 w-4' : size === 'lg' ? 'h-8 w-8' : 'h-6 w-6',
      ]"
    />
    <span v-if="label" class="text-xs text-muted-foreground">{{ label }}</span>
  </div>
</template>
