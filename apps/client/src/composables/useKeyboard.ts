import { onMounted, onBeforeUnmount } from 'vue'

interface ShortcutMap {
  [key: string]: () => void
}

function normalizeKey(e: KeyboardEvent): string {
  const parts: string[] = []
  if (e.ctrlKey || e.metaKey) parts.push('mod')
  if (e.shiftKey) parts.push('shift')
  if (e.altKey) parts.push('alt')
  parts.push(e.key.toLowerCase())
  return parts.join('+')
}

export function useKeyboard(shortcuts: ShortcutMap) {
  function handler(e: KeyboardEvent) {
    const key = normalizeKey(e)
    if (shortcuts[key]) {
      e.preventDefault()
      shortcuts[key]()
    }
  }

  onMounted(() => {
    window.addEventListener('keydown', handler)
  })

  onBeforeUnmount(() => {
    window.removeEventListener('keydown', handler)
  })
}
