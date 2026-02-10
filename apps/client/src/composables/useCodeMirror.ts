import { ref, onMounted, onBeforeUnmount, watch, type Ref } from 'vue'
import { EditorState, Compartment } from '@codemirror/state'
import { EditorView, keymap, lineNumbers, highlightActiveLine } from '@codemirror/view'
import { defaultKeymap } from '@codemirror/commands'
import { syntaxHighlighting, defaultHighlightStyle } from '@codemirror/language'
import { HighlightStyle } from '@codemirror/language'
import { tags } from '@lezer/highlight'
import { useThemeStore } from '@/stores/theme'

// Frost + Aurora syntax colors are shared across light/dark per Nord spec
const nordDarkHighlight = HighlightStyle.define([
  { tag: tags.keyword, color: '#81A1C1' },
  { tag: [tags.keyword, tags.strong], color: '#81A1C1', fontWeight: 'bold' },
  { tag: tags.string, color: '#A3BE8C' },
  { tag: tags.number, color: '#B48EAD' },
  { tag: tags.bool, color: '#81A1C1' },
  { tag: tags.null, color: '#81A1C1' },
  { tag: tags.propertyName, color: '#88C0D0' },
  { tag: tags.comment, color: '#616E88' },
  { tag: tags.operator, color: '#81A1C1' },
  { tag: tags.punctuation, color: '#ECEFF4' },
  { tag: tags.heading, color: '#EBCB8B', fontWeight: 'bold' },
  { tag: tags.url, color: '#8FBCBB' },
  { tag: tags.attributeName, color: '#88C0D0' },
  { tag: tags.variableName, color: '#D08770' },
  { tag: [tags.variableName, tags.special], color: '#D08770', fontStyle: 'italic' },
  { tag: tags.bracket, color: '#ECEFF4' },
])

const nordLightHighlight = HighlightStyle.define([
  { tag: tags.keyword, color: '#5E81AC' },
  { tag: [tags.keyword, tags.strong], color: '#5E81AC', fontWeight: 'bold' },
  { tag: tags.string, color: '#A3BE8C' },
  { tag: tags.number, color: '#B48EAD' },
  { tag: tags.bool, color: '#81A1C1' },
  { tag: tags.null, color: '#81A1C1' },
  { tag: tags.propertyName, color: '#5E81AC' },
  { tag: tags.comment, color: '#4C566A' },
  { tag: tags.operator, color: '#81A1C1' },
  { tag: tags.punctuation, color: '#2E3440' },
  { tag: tags.heading, color: '#D08770', fontWeight: 'bold' },
  { tag: tags.url, color: '#8FBCBB' },
  { tag: tags.attributeName, color: '#5E81AC' },
  { tag: tags.variableName, color: '#D08770' },
  { tag: [tags.variableName, tags.special], color: '#D08770', fontStyle: 'italic' },
  { tag: tags.bracket, color: '#2E3440' },
])

const nordDarkTheme = EditorView.theme({
  '&': {
    backgroundColor: '#2E3440',
    color: '#D8DEE9',
  },
  '.cm-content': {
    caretColor: '#D8DEE9',
    fontFamily: '"JetBrains Mono", monospace',
    fontSize: '13px',
  },
  '.cm-cursor': {
    borderLeftColor: '#D8DEE9',
  },
  '&.cm-focused .cm-selectionBackground, .cm-selectionBackground': {
    backgroundColor: '#434C5E',
  },
  '.cm-activeLine': {
    backgroundColor: '#3B425220',
  },
  '.cm-gutters': {
    backgroundColor: '#2E3440',
    color: '#4C566A',
    border: 'none',
  },
  '.cm-activeLineGutter': {
    backgroundColor: '#3B425220',
  },
  '.cm-tooltip': {
    backgroundColor: '#3B4252',
    border: '1px solid #4C566A',
    color: '#D8DEE9',
  },
  '.cm-tooltip.cm-tooltip-autocomplete': {
    backgroundColor: '#3B4252',
  },
  '.cm-tooltip-autocomplete ul li': {
    color: '#D8DEE9',
  },
  '.cm-tooltip-autocomplete ul li[aria-selected]': {
    backgroundColor: '#434C5E',
    color: '#ECEFF4',
  },
  '.cm-completionIcon': {
    color: '#81A1C1',
  },
  '.cm-completionDetail': {
    color: '#616E88',
    fontStyle: 'italic',
  },
}, { dark: true })

const nordLightTheme = EditorView.theme({
  '&': {
    backgroundColor: '#ECEFF4',
    color: '#2E3440',
  },
  '.cm-content': {
    caretColor: '#2E3440',
    fontFamily: '"JetBrains Mono", monospace',
    fontSize: '13px',
  },
  '.cm-cursor': {
    borderLeftColor: '#2E3440',
  },
  '&.cm-focused .cm-selectionBackground, .cm-selectionBackground': {
    backgroundColor: '#D8DEE9',
  },
  '.cm-activeLine': {
    backgroundColor: '#E5E9F020',
  },
  '.cm-gutters': {
    backgroundColor: '#ECEFF4',
    color: '#4C566A',
    border: 'none',
  },
  '.cm-activeLineGutter': {
    backgroundColor: '#E5E9F020',
  },
  '.cm-tooltip': {
    backgroundColor: '#E5E9F0',
    border: '1px solid #D8DEE9',
    color: '#2E3440',
  },
  '.cm-tooltip.cm-tooltip-autocomplete': {
    backgroundColor: '#E5E9F0',
  },
  '.cm-tooltip-autocomplete ul li': {
    color: '#2E3440',
  },
  '.cm-tooltip-autocomplete ul li[aria-selected]': {
    backgroundColor: '#D8DEE9',
    color: '#2E3440',
  },
  '.cm-completionIcon': {
    color: '#5E81AC',
  },
  '.cm-completionDetail': {
    color: '#4C566A',
    fontStyle: 'italic',
  },
}, { dark: false })

function getThemeExtensions(resolved: 'light' | 'dark') {
  return resolved === 'dark'
    ? [nordDarkTheme, syntaxHighlighting(nordDarkHighlight)]
    : [nordLightTheme, syntaxHighlighting(nordLightHighlight)]
}

export function useCodeMirror(
  container: Ref<HTMLElement | null>,
  content: Ref<string>,
  options: {
    readonly?: boolean
    language?: () => Promise<{ default: unknown }>
    extraExtensions?: () => Promise<{ default: unknown }>
    onChange?: (value: string) => void
  } = {},
) {
  const view = ref<EditorView | null>(null)
  const themeStore = useThemeStore()
  const themeCompartment = new Compartment()

  onMounted(async () => {
    if (!container.value) return

    const extensions = [
      themeCompartment.of(getThemeExtensions(themeStore.resolved)),
      syntaxHighlighting(defaultHighlightStyle),
      lineNumbers(),
      highlightActiveLine(),
      keymap.of(defaultKeymap),
      EditorView.lineWrapping,
    ]

    if (options.readonly) {
      extensions.push(EditorState.readOnly.of(true))
    }

    if (options.onChange) {
      const handler = options.onChange
      extensions.push(
        EditorView.updateListener.of((update) => {
          if (update.docChanged) {
            handler(update.state.doc.toString())
          }
        }),
      )
    }

    if (options.language) {
      try {
        const langModule = await options.language()
        const lang = langModule as { default: unknown }
        if (typeof lang.default === 'function') {
          extensions.push((lang.default as () => unknown)() as never)
        }
      } catch {
        // language extension not available, proceed without
      }
    }

    if (options.extraExtensions) {
      try {
        const extModule = await options.extraExtensions()
        const ext = extModule as { default: unknown }
        if (typeof ext.default === 'function') {
          const result = (ext.default as () => unknown)()
          if (Array.isArray(result)) {
            extensions.push(...(result as never[]))
          } else {
            extensions.push(result as never)
          }
        }
      } catch {
        // extra extensions not available, proceed without
      }
    }

    const state = EditorState.create({
      doc: content.value,
      extensions,
    })

    view.value = new EditorView({
      state,
      parent: container.value,
    })
  })

  watch(content, (newContent) => {
    if (view.value && view.value.state.doc.toString() !== newContent) {
      view.value.dispatch({
        changes: {
          from: 0,
          to: view.value.state.doc.length,
          insert: newContent,
        },
      })
    }
  })

  // Reconfigure CodeMirror theme when app theme changes
  watch(() => themeStore.resolved, (resolved) => {
    if (view.value) {
      view.value.dispatch({
        effects: themeCompartment.reconfigure(getThemeExtensions(resolved)),
      })
    }
  })

  onBeforeUnmount(() => {
    view.value?.destroy()
  })

  return { view }
}
