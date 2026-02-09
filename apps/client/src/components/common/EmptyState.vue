<script setup lang="ts">
import type { Component } from 'vue'

defineProps<{
  icon?: Component
  title: string
  description?: string
}>()
</script>

<template>
  <div class="flex flex-col items-center justify-center gap-3 p-8 text-center">
    <component :is="icon" v-if="icon" class="h-12 w-12 text-muted-foreground/50" />
    <h3 class="text-sm font-medium text-foreground">{{ title }}</h3>
    <p v-if="description" class="text-sm text-muted-foreground">{{ description }}</p>
    <slot />
  </div>
</template>
