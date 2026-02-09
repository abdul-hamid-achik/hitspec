<script setup lang="ts">
import { ref } from 'vue'
import { Search } from 'lucide-vue-next'

const query = ref('')
const emit = defineEmits<{ search: [value: string] }>()

function onInput(e: Event) {
  query.value = (e.target as HTMLInputElement).value
  emit('search', query.value)
}
</script>

<template>
  <div class="relative">
    <Search :size="14" class="absolute left-2.5 top-1/2 -translate-y-1/2 text-nord-3" />
    <input
      type="text"
      placeholder="Search files..."
      :value="query"
      class="w-full rounded-md border border-border bg-input py-1.5 pl-8 pr-3 text-sm text-foreground placeholder:text-nord-3 focus:outline-none focus:ring-1 focus:ring-ring"
      @input="onInput"
    />
  </div>
</template>
