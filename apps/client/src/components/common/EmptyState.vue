<script setup lang="ts">
import type { Component } from 'vue'

defineProps<{
  icon?: Component
  title: string
  description?: string
  compact?: boolean
}>()
</script>

<template>
  <div
    :class="[
      'flex flex-col items-center justify-center text-center',
      compact ? 'gap-2 p-4' : 'gap-3 p-8',
    ]"
  >
    <div v-if="icon" class="rounded-xl bg-surface p-3">
      <component
        :is="icon"
        :class="[
          'text-muted-foreground/40',
          compact ? 'h-8 w-8' : 'h-10 w-10',
        ]"
      />
    </div>
    <div class="space-y-1">
      <h3 :class="['font-medium text-foreground', compact ? 'text-xs' : 'text-sm']">{{ title }}</h3>
      <p v-if="description" :class="['text-muted-foreground/70', compact ? 'text-xs' : 'text-sm']">
        {{ description }}
      </p>
    </div>
    <slot />
  </div>
</template>
