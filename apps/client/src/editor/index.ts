import { hitspecLanguage } from './hitspec-language'
import { hitspecCompletions } from './hitspec-completions'
import { autocompletion } from '@codemirror/autocomplete'
import type { Extension } from '@codemirror/state'

/**
 * Returns a CodeMirror extension bundle for .http/.hitspec files.
 * Includes syntax highlighting and autocomplete.
 */
export function hitspec(): Extension {
  return [
    hitspecLanguage,
    autocompletion({
      override: [hitspecCompletions],
      activateOnTyping: true,
      icons: true,
    }),
  ]
}
