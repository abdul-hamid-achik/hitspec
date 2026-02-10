import { ref, onMounted, onBeforeUnmount, watch, type Ref } from 'vue'
import { EditorState } from '@codemirror/state'
import { EditorView, keymap, lineNumbers, highlightActiveLine } from '@codemirror/view'
import { defaultKeymap } from '@codemirror/commands'
import { syntaxHighlighting, defaultHighlightStyle } from '@codemirror/language'
import { HighlightStyle } from '@codemirror/language'
import { tags } from '@lezer/highlight'

const nordHighlight = HighlightStyle.define([
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

const nordTheme = EditorView.theme({
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
})

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

  onMounted(async () => {
    if (!container.value) return

    const extensions = [
      nordTheme,
      syntaxHighlighting(nordHighlight),
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

  onBeforeUnmount(() => {
    view.value?.destroy()
  })

  return { view }
}
