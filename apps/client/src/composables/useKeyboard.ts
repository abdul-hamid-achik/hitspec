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

function isEditableTarget(target: EventTarget | null): boolean {
  if (!target || !(target instanceof HTMLElement)) return false
  const tag = target.tagName
  if (tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT') return true
  return target.isContentEditable
}

export function useKeyboard(shortcuts: ShortcutMap) {
  function handler(e: KeyboardEvent) {
    const key = normalizeKey(e)
    const action = shortcuts[key]
    if (!action) return

    // For shortcuts without modifiers (like Escape), skip when the user is
    // typing in an input so we don't steal keystrokes.  Modifier shortcuts
    // (Cmd/Ctrl+...) are always safe to intercept.
    const hasModifier = e.ctrlKey || e.metaKey || e.altKey
    if (!hasModifier && key !== 'escape' && isEditableTarget(e.target)) return

    // Don't preventDefault on Escape -- let Reka UI dialogs close natively
    if (key !== 'escape') {
      e.preventDefault()
    }
    action()
  }

  onMounted(() => {
    window.addEventListener('keydown', handler)
  })

  onBeforeUnmount(() => {
    window.removeEventListener('keydown', handler)
  })
}
