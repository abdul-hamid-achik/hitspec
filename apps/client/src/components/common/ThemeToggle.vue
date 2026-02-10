<script setup lang="ts">
import { Sun, Moon, Monitor } from 'lucide-vue-next'
import { useThemeStore } from '@/stores/theme'

const theme = useThemeStore()
</script>

<template>
  <button
    :aria-label="`Theme: ${theme.mode}. Click to switch.`"
    :title="`Theme: ${theme.mode}`"
    class="rounded p-1 text-muted-foreground transition-colors hover:bg-surface-hover hover:text-foreground"
    @click="theme.toggle()"
  >
    <Sun v-if="theme.resolved === 'light' && theme.mode !== 'system'" class="h-4 w-4" />
    <Moon v-else-if="theme.resolved === 'dark' && theme.mode !== 'system'" class="h-4 w-4" />
    <Monitor v-else class="h-4 w-4" />
  </button>
</template>
