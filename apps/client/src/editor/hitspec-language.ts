import {
  StreamLanguage,
  type StringStream,
  type StreamParser,
} from '@codemirror/language'

interface HitspecState {
  /** Current parsing context */
  context:
    | 'top' // file level (variables, separators, comments)
    | 'request' // after method line, reading headers/url
    | 'body' // inside request body (raw JSON, XML, etc.)
    | 'assertions' // inside >>> ... <<< assertion block
    | 'capture' // inside >>>capture ... <<<
    | 'shell' // inside >>>shell ... <<<
    | 'db' // inside >>>db ... <<<
    | 'multipart' // inside >>>multipart ... <<<
    | 'graphql' // inside >>>graphql ... <<<
    | 'variables' // inside >>>variables ... <<<
  /** Whether we're at the start of a line */
  sol: boolean
}

const HTTP_METHODS = /^(GET|POST|PUT|PATCH|DELETE|HEAD|OPTIONS|TRACE|CONNECT|WS)\b/

const BLOCK_END = /^<<<\s*$/

const ASSERTION_OPERATORS =
  /^(==|!=|>=|<=|>|<|contains|!contains|startsWith|endsWith|matches|exists|!exists|length|includes|!includes|in|!in|type|each|schema|snapshot)\b/

const DIRECTIVES =
  /^@(name|description|tags|skip|only|timeout|retry|retryDelay|retrydelay|depends|auth|before|after|db|waitfor|stress\.weight|stress\.think|stress\.skip|stress\.setup|stress\.teardown)\b/i

function tokenize(stream: StringStream, state: HitspecState): string | null {
  // Track start of line
  state.sol = stream.sol()

  // --- Block end <<<  ---
  if (stream.sol() && stream.match(BLOCK_END)) {
    const prevCtx = state.context
    state.context = 'top'
    // Give a style hint based on what block we're closing
    if (prevCtx === 'assertions') return 'keyword'
    if (prevCtx === 'capture') return 'keyword'
    return 'keyword'
  }

  // --- Block start >>>  ---
  if (stream.sol() && stream.match(/^>>>/)) {
    const rest = stream.match(/^(\w+)?\s*$/, false)
    const blockType = rest ? (stream.match(/^(\w+)\s*$/) as RegExpMatchArray)?.[1]?.toLowerCase() : ''

    // Consume the rest of line
    if (!rest) stream.skipToEnd()

    switch (blockType) {
      case 'capture':
        state.context = 'capture'
        break
      case 'shell':
        state.context = 'shell'
        break
      case 'db':
        state.context = 'db'
        break
      case 'multipart':
        state.context = 'multipart'
        break
      case 'graphql':
        state.context = 'graphql'
        break
      case 'variables':
        state.context = 'variables'
        break
      default:
        state.context = 'assertions'
        break
    }
    return 'keyword'
  }

  // --- Context-specific tokenization ---

  // Shell block: highlight as plain text with variable interpolation
  if (state.context === 'shell') {
    if (stream.match(/^\{\{/)) {
      stream.match(/[^}]*\}\}/)
      return 'variableName.special'
    }
    stream.skipToEnd()
    return 'string'
  }

  // DB block
  if (state.context === 'db') {
    if (stream.match(/^\{\{/)) {
      stream.match(/[^}]*\}\}/)
      return 'variableName.special'
    }
    if (stream.sol() && stream.match(/^query\b/)) return 'keyword'
    if (stream.sol() && stream.match(/^expect\b/)) return 'keyword'
    if (stream.match(ASSERTION_OPERATORS)) return 'operator'
    if (stream.match(/^"([^"\\]|\\.)*"/)) return 'string'
    if (stream.match(/^'([^'\\]|\\.)*'/)) return 'string'
    if (stream.match(/^-?\d+(\.\d+)?/)) return 'number'
    if (stream.match(/^(true|false)\b/)) return 'bool'
    if (stream.match(/^null\b/)) return 'null'
    stream.next()
    return null
  }

  // GraphQL block
  if (state.context === 'graphql' || state.context === 'variables') {
    if (stream.match(/^\{\{/)) {
      stream.match(/[^}]*\}\}/)
      return 'variableName.special'
    }
    if (stream.match(/^"([^"\\]|\\.)*"/)) return 'string'
    if (stream.match(/^#.*/)) return 'comment'
    if (stream.match(/^(query|mutation|subscription|fragment|on|type|input|enum|interface|union|scalar|schema|extend|directive|implements)\b/)) return 'keyword'
    if (stream.match(/^\$\w+/)) return 'variableName'
    if (stream.match(/^@\w+/)) return 'attributeName'
    if (stream.match(/^-?\d+(\.\d+)?/)) return 'number'
    if (stream.match(/^(true|false)\b/)) return 'bool'
    if (stream.match(/^null\b/)) return 'null'
    stream.next()
    return null
  }

  // Multipart block
  if (state.context === 'multipart') {
    if (stream.sol() && stream.match(/^(file|field)\b/)) return 'keyword'
    if (stream.match(/^\{\{/)) {
      stream.match(/[^}]*\}\}/)
      return 'variableName.special'
    }
    if (stream.match(/^"([^"\\]|\\.)*"/)) return 'string'
    if (stream.match(/^@/)) return 'operator'
    stream.next()
    return null
  }

  // Assertion block
  if (state.context === 'assertions') {
    if (stream.sol() && stream.match(/^expect\b/)) return 'keyword'
    if (stream.match(/^\{\{/)) {
      stream.match(/[^}]*\}\}/)
      return 'variableName.special'
    }
    if (stream.match(ASSERTION_OPERATORS)) return 'operator'
    // Assertion subjects: status, header.*, body.*, jsonpath, duration, size, p50, p95, p99
    if (stream.sol()) {
      stream.eatSpace()
      if (stream.match(/^expect\b/)) return 'keyword'
    }
    if (stream.match(/^(status|duration|size|p50|p95|p99)\b/)) return 'attributeName'
    if (stream.match(/^(header|body|jsonpath)\b/)) return 'attributeName'
    if (stream.match(/^"([^"\\]|\\.)*"/)) return 'string'
    if (stream.match(/^'([^'\\]|\\.)*'/)) return 'string'
    if (stream.match(/^-?\d+(\.\d+)?/)) return 'number'
    if (stream.match(/^(true|false)\b/)) return 'bool'
    if (stream.match(/^null\b/)) return 'null'
    if (stream.match(/^\[/)) return 'bracket'
    if (stream.match(/^\]/)) return 'bracket'
    stream.next()
    return null
  }

  // Capture block
  if (state.context === 'capture') {
    if (stream.match(/^from\b/)) return 'keyword'
    if (stream.match(/^\{\{/)) {
      stream.match(/[^}]*\}\}/)
      return 'variableName.special'
    }
    if (stream.match(/^(header|body|status|duration)\b/)) return 'attributeName'
    if (stream.match(/^\w[\w.-]*/)) return 'variableName'
    stream.next()
    return null
  }

  // --- Top-level / request context ---

  // Comments: # or //
  if (stream.sol() && stream.match(/^#\s*@/)) {
    // Annotation in comment: # @name value
    stream.backUp(stream.current().length)
    stream.next() // skip #
    stream.eatSpace()
    if (stream.match(DIRECTIVES)) {
      // Read rest as annotation value
      stream.skipToEnd()
      return 'attributeName'
    }
    stream.skipToEnd()
    return 'comment'
  }
  if (stream.sol() && (stream.match(/^#/) || stream.match(/^\/\//))) {
    stream.skipToEnd()
    return 'comment'
  }

  // Request separator: ###
  if (stream.sol() && stream.match(/^###/)) {
    stream.skipToEnd()
    return 'heading'
  }

  // Variables: @name = value
  if (stream.sol() && stream.peek() === '@') {
    // Check if this is a directive or a variable assignment
    const saved = stream.pos
    stream.next() // skip @
    if (stream.match(/^\w[\w.-]*/)) {
      stream.eatSpace()
      if (stream.peek() === '=') {
        // Variable assignment: @varName = value
        stream.pos = saved
        stream.next() // @
        stream.match(/^\w[\w.-]*/)
        return 'variableName.definition'
      }
      // Directive: @name value
      stream.pos = saved
      stream.next()
      stream.match(/^\w[\w.-]*/)
      stream.skipToEnd()
      return 'attributeName'
    }
    stream.pos = saved
  }

  // Variable references: {{...}}
  if (stream.match(/^\{\{/)) {
    // Read function calls like $now() inside {{}}
    stream.match(/[^}]*\}\}/)
    return 'variableName.special'
  }

  // HTTP Methods at start of line -> enter request context
  if (stream.sol() && stream.match(HTTP_METHODS)) {
    state.context = 'request'
    return 'keyword strong'
  }

  // In request context, the URL follows the method
  if (state.context === 'request' && !stream.sol()) {
    // URL part
    if (stream.match(/^\{\{/)) {
      stream.match(/[^}]*\}\}/)
      return 'variableName.special'
    }
    if (stream.match(/^https?:\/\/\S*/)) return 'url'
    if (stream.match(/^\S+/)) return 'url'
    stream.next()
    return 'url'
  }

  // Query params: ? key = value or & key = value
  if (stream.sol() && stream.match(/^[?&]\s*/)) {
    return 'operator'
  }

  // Headers: Key: Value  (identifier followed by colon)
  if (stream.sol() && stream.match(/^[\w-]+(?=\s*:)/)) {
    return 'propertyName'
  }
  if (stream.match(/^:\s*/)) {
    // After header colon, rest is value
    if (stream.match(/^\{\{/)) {
      stream.match(/[^}]*\}\}/)
      return 'variableName.special'
    }
    stream.skipToEnd()
    return 'string'
  }

  // = in variable assignments
  if (stream.match(/^=/)) {
    return 'operator'
  }

  // Strings
  if (stream.match(/^"([^"\\]|\\.)*"/)) return 'string'
  if (stream.match(/^'([^'\\]|\\.)*'/)) return 'string'

  // Numbers
  if (stream.match(/^-?\d+(\.\d+)?/)) return 'number'

  // Booleans
  if (stream.match(/^(true|false)\b/)) return 'bool'

  // Null
  if (stream.match(/^null\b/)) return 'null'

  // Body context: if we're past headers and before assertions
  // Just pass through as default text
  stream.next()
  return null
}

const hitspecParser: StreamParser<HitspecState> = {
  startState(): HitspecState {
    return { context: 'top', sol: true }
  },
  token: tokenize,
  blankLine(state) {
    // Blank lines in request context move to body
    if (state.context === 'request') {
      state.context = 'body'
    }
  },
}

export const hitspecLanguage = StreamLanguage.define(hitspecParser)
