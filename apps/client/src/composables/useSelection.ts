import { ref, computed, type Ref, type ComputedRef } from 'vue'

interface Identifiable {
  id: number | string
}

export function useSelection<T extends Identifiable>(items: Ref<T[]> | ComputedRef<T[]>) {
  const selectedIds = ref<Set<number | string>>(new Set())
  let lastClickedIndex: number | null = null

  const selectedItems = computed(() =>
    items.value.filter(item => selectedIds.value.has(item.id)),
  )

  const allSelected = computed(() => {
    if (items.value.length === 0) return false
    return items.value.every(item => selectedIds.value.has(item.id))
  })

  const someSelected = computed(() => {
    if (items.value.length === 0) return false
    const count = items.value.filter(item => selectedIds.value.has(item.id)).length
    return count > 0 && count < items.value.length
  })

  function isSelected(item: T): boolean {
    return selectedIds.value.has(item.id)
  }

  function toggle(item: T, event?: MouseEvent) {
    const newSet = new Set(selectedIds.value)
    const idx = items.value.findIndex(i => i.id === item.id)

    if (event?.shiftKey && lastClickedIndex !== null && idx !== -1) {
      // Range select
      const start = Math.min(lastClickedIndex, idx)
      const end = Math.max(lastClickedIndex, idx)
      for (let i = start; i <= end; i++) {
        newSet.add(items.value[i].id)
      }
    } else {
      if (newSet.has(item.id)) {
        newSet.delete(item.id)
      } else {
        newSet.add(item.id)
      }
    }

    lastClickedIndex = idx
    selectedIds.value = newSet
  }

  function selectAll() {
    selectedIds.value = new Set(items.value.map(i => i.id))
  }

  function deselectAll() {
    selectedIds.value = new Set()
    lastClickedIndex = null
  }

  function toggleAll() {
    if (allSelected.value) {
      deselectAll()
    } else {
      selectAll()
    }
  }

  return {
    selectedIds,
    selectedItems,
    allSelected,
    someSelected,
    isSelected,
    toggle,
    selectAll,
    deselectAll,
    toggleAll,
  }
}
