<script setup lang="ts">
import { Plus, Trash2 } from 'lucide-vue-next'

const props = defineProps<{
  items: { key: string; value: string; enabled?: boolean }[]
  keyPlaceholder?: string
  valuePlaceholder?: string
}>()

const emit = defineEmits<{
  'update:items': [items: { key: string; value: string; enabled?: boolean }[]]
}>()

function addItem() {
  emit('update:items', [...props.items, { key: '', value: '', enabled: true }])
}

function removeItem(index: number) {
  const updated = [...props.items]
  updated.splice(index, 1)
  emit('update:items', updated)
}

function updateItem(index: number, field: 'key' | 'value', val: string) {
  const updated = [...props.items]
  updated[index] = { ...updated[index], [field]: val }
  emit('update:items', updated)
}

function toggleItem(index: number) {
  const updated = [...props.items]
  updated[index] = { ...updated[index], enabled: !updated[index].enabled }
  emit('update:items', updated)
}
</script>

<template>
  <div class="space-y-1">
    <div
      v-for="(item, i) in items"
      :key="i"
      class="flex items-center gap-1"
    >
      <input
        type="checkbox"
        :checked="item.enabled !== false"
        class="accent-accent"
        @change="toggleItem(i)"
      />
      <input
        :value="item.key"
        :placeholder="keyPlaceholder ?? 'Key'"
        class="flex-1 rounded border border-border bg-background px-2 py-1 font-mono text-xs text-foreground placeholder:text-muted-foreground/50"
        @input="updateItem(i, 'key', ($event.target as HTMLInputElement).value)"
      />
      <input
        :value="item.value"
        :placeholder="valuePlaceholder ?? 'Value'"
        class="flex-1 rounded border border-border bg-background px-2 py-1 font-mono text-xs text-foreground placeholder:text-muted-foreground/50"
        @input="updateItem(i, 'value', ($event.target as HTMLInputElement).value)"
      />
      <button class="p-1 text-muted-foreground hover:text-destructive" @click="removeItem(i)">
        <Trash2 class="h-3.5 w-3.5" />
      </button>
    </div>
    <button class="flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground" @click="addItem">
      <Plus class="h-3 w-3" />
      Add
    </button>
  </div>
</template>
