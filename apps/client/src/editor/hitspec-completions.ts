import type { CompletionContext, CompletionResult, Completion } from '@codemirror/autocomplete'

// --- Completion data ---

const HTTP_METHODS: Completion[] = [
  { label: 'GET', type: 'keyword', detail: 'HTTP GET request' },
  { label: 'POST', type: 'keyword', detail: 'HTTP POST request' },
  { label: 'PUT', type: 'keyword', detail: 'HTTP PUT request' },
  { label: 'PATCH', type: 'keyword', detail: 'HTTP PATCH request' },
  { label: 'DELETE', type: 'keyword', detail: 'HTTP DELETE request' },
  { label: 'HEAD', type: 'keyword', detail: 'HTTP HEAD request' },
  { label: 'OPTIONS', type: 'keyword', detail: 'HTTP OPTIONS request' },
  { label: 'WS', type: 'keyword', detail: 'WebSocket connection' },
]

const SECTION_MARKERS: Completion[] = [
  { label: '>>>', type: 'keyword', detail: 'assertions block', apply: '>>>\n\n<<<', boost: 2 },
  { label: '>>>capture', type: 'keyword', detail: 'capture variables', apply: '>>>capture\n\n<<<' },
  { label: '>>>shell', type: 'keyword', detail: 'shell commands', apply: '>>>shell\n\n<<<' },
  { label: '>>>db', type: 'keyword', detail: 'database assertions', apply: '>>>db\n\n<<<' },
  { label: '>>>multipart', type: 'keyword', detail: 'multipart body', apply: '>>>multipart\n\n<<<' },
  { label: '>>>graphql', type: 'keyword', detail: 'GraphQL body', apply: '>>>graphql\n\n<<<' },
  { label: '>>>variables', type: 'keyword', detail: 'GraphQL variables', apply: '>>>variables\n\n<<<' },
]

const DIRECTIVES: Completion[] = [
  { label: '@name', type: 'property', detail: 'request identifier', apply: '@name ' },
  { label: '@description', type: 'property', detail: 'request description', apply: '@description ' },
  { label: '@tags', type: 'property', detail: 'comma-separated tags', apply: '@tags ' },
  { label: '@skip', type: 'property', detail: 'skip this request', apply: '@skip' },
  { label: '@only', type: 'property', detail: 'run only this request', apply: '@only' },
  { label: '@timeout', type: 'property', detail: 'timeout in ms', apply: '@timeout ' },
  { label: '@retry', type: 'property', detail: 'retry count', apply: '@retry ' },
  { label: '@retryDelay', type: 'property', detail: 'delay between retries (ms)', apply: '@retryDelay ' },
  { label: '@depends', type: 'property', detail: 'request dependencies', apply: '@depends ' },
  { label: '@auth', type: 'property', detail: 'auth configuration', apply: '@auth ' },
  { label: '@before', type: 'property', detail: 'pre-request hook', apply: '@before ' },
  { label: '@after', type: 'property', detail: 'post-request hook', apply: '@after ' },
  { label: '@db', type: 'property', detail: 'database connection', apply: '@db ' },
  { label: '@waitfor', type: 'property', detail: 'wait for URL to be ready', apply: '@waitfor ' },
  { label: '@stress.weight', type: 'property', detail: 'stress test weight', apply: '@stress.weight ' },
  { label: '@stress.think', type: 'property', detail: 'think time in ms', apply: '@stress.think ' },
  { label: '@stress.skip', type: 'property', detail: 'exclude from stress test', apply: '@stress.skip' },
  { label: '@stress.setup', type: 'property', detail: 'run once before stress test', apply: '@stress.setup' },
  { label: '@stress.teardown', type: 'property', detail: 'run once after stress test', apply: '@stress.teardown' },
]

const AUTH_TYPES: Completion[] = [
  { label: 'bearer', type: 'enum', detail: 'Bearer token auth', apply: 'bearer {{token}}' },
  { label: 'basic', type: 'enum', detail: 'Basic auth', apply: 'basic {{username}} {{password}}' },
  { label: 'apikey', type: 'enum', detail: 'API key in header', apply: 'apikey {{headerName}} {{key}}' },
  { label: 'apikey-query', type: 'enum', detail: 'API key in query param', apply: 'apikey-query {{paramName}} {{key}}' },
  { label: 'digest', type: 'enum', detail: 'Digest auth', apply: 'digest {{username}} {{password}}' },
  { label: 'aws', type: 'enum', detail: 'AWS Signature V4', apply: 'aws {{accessKey}} {{secretKey}} {{region}} {{service}}' },
  { label: 'oauth2 client_credentials', type: 'enum', detail: 'OAuth2 client credentials', apply: 'oauth2 client_credentials {{tokenUrl}} {{clientId}} {{clientSecret}}' },
  { label: 'oauth2 password', type: 'enum', detail: 'OAuth2 password grant', apply: 'oauth2 password {{tokenUrl}} {{clientId}} {{clientSecret}} {{username}} {{password}}' },
]

const COMMON_HEADERS: Completion[] = [
  { label: 'Content-Type', type: 'property', detail: 'request content type' },
  { label: 'Accept', type: 'property', detail: 'acceptable response types' },
  { label: 'Authorization', type: 'property', detail: 'auth credentials' },
  { label: 'User-Agent', type: 'property', detail: 'client identifier' },
  { label: 'Cache-Control', type: 'property', detail: 'caching directives' },
  { label: 'Cookie', type: 'property', detail: 'request cookies' },
  { label: 'X-Request-ID', type: 'property', detail: 'request tracing ID' },
  { label: 'X-API-Key', type: 'property', detail: 'API key header' },
  { label: 'If-None-Match', type: 'property', detail: 'conditional request' },
  { label: 'If-Modified-Since', type: 'property', detail: 'conditional request' },
  { label: 'Origin', type: 'property', detail: 'request origin' },
  { label: 'Referer', type: 'property', detail: 'referring URL' },
]

const CONTENT_TYPES: Completion[] = [
  { label: 'application/json', type: 'text' },
  { label: 'application/xml', type: 'text' },
  { label: 'application/x-www-form-urlencoded', type: 'text' },
  { label: 'multipart/form-data', type: 'text' },
  { label: 'text/plain', type: 'text' },
  { label: 'text/html', type: 'text' },
  { label: 'text/xml', type: 'text' },
  { label: 'application/octet-stream', type: 'text' },
  { label: 'application/graphql', type: 'text' },
]

const ASSERTION_OPERATORS: Completion[] = [
  { label: '==', type: 'operator', detail: 'equals' },
  { label: '!=', type: 'operator', detail: 'not equals' },
  { label: '>', type: 'operator', detail: 'greater than' },
  { label: '>=', type: 'operator', detail: 'greater or equal' },
  { label: '<', type: 'operator', detail: 'less than' },
  { label: '<=', type: 'operator', detail: 'less or equal' },
  { label: 'contains', type: 'operator', detail: 'string contains' },
  { label: '!contains', type: 'operator', detail: 'string does not contain' },
  { label: 'startsWith', type: 'operator', detail: 'string starts with' },
  { label: 'endsWith', type: 'operator', detail: 'string ends with' },
  { label: 'matches', type: 'operator', detail: 'regex match' },
  { label: 'exists', type: 'operator', detail: 'value exists (not null)' },
  { label: '!exists', type: 'operator', detail: 'value does not exist' },
  { label: 'length', type: 'operator', detail: 'length equals' },
  { label: 'includes', type: 'operator', detail: 'array includes value' },
  { label: '!includes', type: 'operator', detail: 'array does not include' },
  { label: 'in', type: 'operator', detail: 'value is in array' },
  { label: '!in', type: 'operator', detail: 'value is not in array' },
  { label: 'type', type: 'operator', detail: 'type check (string, number, boolean, array, object, null)' },
  { label: 'each', type: 'operator', detail: 'assert each array element' },
  { label: 'schema', type: 'operator', detail: 'JSON Schema validation' },
  { label: 'snapshot', type: 'operator', detail: 'snapshot comparison' },
]

const ASSERTION_SUBJECTS: Completion[] = [
  { label: 'status', type: 'variable', detail: 'HTTP status code' },
  { label: 'duration', type: 'variable', detail: 'response time in ms' },
  { label: 'body', type: 'variable', detail: 'response body' },
  { label: 'body.', type: 'variable', detail: 'JSON path on body', apply: 'body.' },
  { label: 'header', type: 'variable', detail: 'response header', apply: 'header ' },
  { label: 'jsonpath', type: 'variable', detail: 'JSONPath query', apply: 'jsonpath ' },
  { label: 'p50', type: 'variable', detail: '50th percentile latency' },
  { label: 'p95', type: 'variable', detail: '95th percentile latency' },
  { label: 'p99', type: 'variable', detail: '99th percentile latency' },
]

const TYPE_VALUES: Completion[] = [
  { label: 'string', type: 'type' },
  { label: 'number', type: 'type' },
  { label: 'boolean', type: 'type' },
  { label: 'array', type: 'type' },
  { label: 'object', type: 'type' },
  { label: 'null', type: 'type' },
]

const BUILTIN_FUNCTIONS: Completion[] = [
  { label: '$now()', type: 'function', detail: 'current time (RFC3339)' },
  { label: '$timestamp()', type: 'function', detail: 'unix timestamp (seconds)' },
  { label: '$timestampMs()', type: 'function', detail: 'unix timestamp (milliseconds)' },
  { label: '$uuid()', type: 'function', detail: 'random UUID v4' },
  { label: '$random(min, max)', type: 'function', detail: 'random integer', apply: '$random(0, 100)' },
  { label: '$randomString(len)', type: 'function', detail: 'random string', apply: '$randomString(16)' },
  { label: '$randomEmail()', type: 'function', detail: 'random email address' },
  { label: '$randomAlphanumeric(len)', type: 'function', detail: 'random alphanumeric', apply: '$randomAlphanumeric(8)' },
  { label: '$base64(val)', type: 'function', detail: 'base64 encode', apply: '$base64(' },
  { label: '$base64Decode(val)', type: 'function', detail: 'base64 decode', apply: '$base64Decode(' },
  { label: '$md5(val)', type: 'function', detail: 'MD5 hash', apply: '$md5(' },
  { label: '$sha256(val)', type: 'function', detail: 'SHA-256 hash', apply: '$sha256(' },
  { label: '$urlEncode(val)', type: 'function', detail: 'URL encode', apply: '$urlEncode(' },
  { label: '$urlDecode(val)', type: 'function', detail: 'URL decode', apply: '$urlDecode(' },
  { label: '$date(format)', type: 'function', detail: 'formatted date', apply: '$date(2006-01-02)' },
  { label: '$json(val)', type: 'function', detail: 'raw JSON value', apply: '$json(' },
  { label: '$env(VAR)', type: 'function', detail: 'environment variable', apply: '$env(' },
]

// --- Context detection ---

interface LineContext {
  /** What block we're currently inside */
  block: 'top' | 'assertions' | 'capture' | 'shell' | 'db' | 'multipart' | 'graphql' | 'variables'
  /** The full text of the current line */
  lineText: string
  /** Text before cursor on the current line */
  beforeCursor: string
}

function getLineContext(context: CompletionContext): LineContext {
  const { state, pos } = context
  const line = state.doc.lineAt(pos)
  const lineText = line.text
  const beforeCursor = lineText.slice(0, pos - line.from)

  // Walk backwards from cursor to determine which block we're in
  let block: LineContext['block'] = 'top'
  for (let i = line.number - 1; i >= 1; i--) {
    const text = state.doc.line(i).text.trim()
    if (/^<<</.test(text)) {
      block = 'top'
      continue
    }
    if (/^>>>capture/.test(text)) { block = 'capture'; break }
    if (/^>>>shell/.test(text)) { block = 'shell'; break }
    if (/^>>>db/.test(text)) { block = 'db'; break }
    if (/^>>>multipart/.test(text)) { block = 'multipart'; break }
    if (/^>>>graphql/.test(text)) { block = 'graphql'; break }
    if (/^>>>variables/.test(text)) { block = 'variables'; break }
    if (/^>>>/.test(text)) { block = 'assertions'; break }
  }

  // Also check the current line itself
  if (/^>>>capture/.test(lineText.trim())) block = 'capture'
  else if (/^>>>shell/.test(lineText.trim())) block = 'shell'
  else if (/^>>>db/.test(lineText.trim())) block = 'db'
  else if (/^>>>multipart/.test(lineText.trim())) block = 'multipart'
  else if (/^>>>graphql/.test(lineText.trim())) block = 'graphql'
  else if (/^>>>variables/.test(lineText.trim())) block = 'variables'
  else if (/^>>>/.test(lineText.trim())) block = 'assertions'

  return { block, lineText, beforeCursor }
}

// --- Main completion function ---

export function hitspecCompletions(context: CompletionContext): CompletionResult | null {
  const { beforeCursor, block } = getLineContext(context)

  // Inside {{ }} -> offer built-in functions
  const varMatch = beforeCursor.match(/\{\{\s*(\$?\w*)$/)
  if (varMatch) {
    const prefix = varMatch[1]
    return {
      from: context.pos - prefix.length,
      options: BUILTIN_FUNCTIONS,
      validFor: /^[$\w]*/,
    }
  }

  // Assertion block completions
  if (block === 'assertions') {
    return assertionCompletions(context, beforeCursor)
  }

  // Capture block completions
  if (block === 'capture') {
    return captureCompletions(context, beforeCursor)
  }

  // DB block
  if (block === 'db') {
    return dbCompletions(context, beforeCursor)
  }

  // Top-level completions
  return topLevelCompletions(context, beforeCursor)
}

function topLevelCompletions(context: CompletionContext, beforeCursor: string): CompletionResult | null {
  const trimmed = beforeCursor.trimStart()

  // After @auth -> auth types
  if (/^#?\s*@auth\s+/.test(trimmed) || /^@auth\s+/.test(trimmed)) {
    const word = context.matchBefore(/\w*/)
    if (!word) return null
    return { from: word.from, options: AUTH_TYPES, validFor: /^\w*/ }
  }

  // Directives starting with # @
  if (/^#\s*@\w*$/.test(trimmed)) {
    const atMatch = beforeCursor.match(/@(\w*)$/)
    if (atMatch) {
      return {
        from: context.pos - atMatch[0].length,
        options: DIRECTIVES,
        validFor: /^@[\w.]*/,
      }
    }
  }

  // Directives starting with @
  if (/^@[\w.]*$/.test(trimmed)) {
    return {
      from: context.pos - trimmed.length,
      options: DIRECTIVES,
      validFor: /^@[\w.]*/,
    }
  }

  // Section markers starting with >
  if (/^>{1,3}\w*$/.test(trimmed)) {
    return {
      from: context.pos - trimmed.length,
      options: SECTION_MARKERS,
      validFor: /^>{0,3}\w*/,
    }
  }

  // HTTP methods at start of line
  if (/^[A-Z]*$/.test(trimmed) && trimmed.length > 0) {
    return {
      from: context.pos - trimmed.length,
      options: HTTP_METHODS,
      validFor: /^[A-Z]*/,
    }
  }

  // Header names at start of line (after an HTTP method line)
  if (/^[\w-]*$/.test(trimmed) && trimmed.length >= 1) {
    const word = context.matchBefore(/[\w-]+/)
    if (word) {
      return {
        from: word.from,
        options: [...COMMON_HEADERS, ...HTTP_METHODS],
        validFor: /^[\w-]*/,
      }
    }
  }

  // Header values (after header-name: )
  const headerValueMatch = beforeCursor.match(/^(Content-Type|Accept)\s*:\s*(.*)$/i)
  if (headerValueMatch) {
    const valuePrefix = headerValueMatch[2]
    return {
      from: context.pos - valuePrefix.length,
      options: CONTENT_TYPES,
      validFor: /^[\w/.-]*/,
    }
  }

  // If no explicit match but user is typing, offer common start-of-line options
  if (!context.explicit) return null

  // Offer all top-level options on explicit invoke
  return {
    from: context.pos,
    options: [...HTTP_METHODS, ...SECTION_MARKERS, ...DIRECTIVES],
  }
}

function assertionCompletions(context: CompletionContext, beforeCursor: string): CompletionResult | null {
  const trimmed = beforeCursor.trimStart()

  // "expect" keyword at start
  if (/^e\w*$/.test(trimmed)) {
    return {
      from: context.pos - trimmed.length,
      options: [{ label: 'expect', type: 'keyword', detail: 'assertion', apply: 'expect ' }],
      validFor: /^\w*/,
    }
  }

  // After "expect " -> assertion subjects
  if (/^expect\s+[\w.[\]]*$/.test(trimmed)) {
    const subjectMatch = trimmed.match(/^expect\s+([\w.[\]]*)$/)
    if (subjectMatch) {
      const prefix = subjectMatch[1]
      return {
        from: context.pos - prefix.length,
        options: ASSERTION_SUBJECTS,
        validFor: /^[\w.[\]]*/,
      }
    }
  }

  // After subject -> assertion operators
  const operatorMatch = trimmed.match(/^expect\s+\S+\s+(!?\w*)$/)
  if (operatorMatch) {
    const prefix = operatorMatch[1]
    return {
      from: context.pos - prefix.length,
      options: ASSERTION_OPERATORS,
      validFor: /^!?\w*/,
    }
  }

  // After "type" operator -> type values
  const typeMatch = trimmed.match(/^expect\s+\S+\s+type\s+(\w*)$/)
  if (typeMatch) {
    const prefix = typeMatch[1]
    return {
      from: context.pos - prefix.length,
      options: TYPE_VALUES,
      validFor: /^\w*/,
    }
  }

  // Explicit invoke in assertion block
  if (context.explicit) {
    return {
      from: context.pos,
      options: [{ label: 'expect', type: 'keyword', apply: 'expect ' }],
    }
  }

  return null
}

function captureCompletions(context: CompletionContext, beforeCursor: string): CompletionResult | null {
  const trimmed = beforeCursor.trimStart()

  // After variable name and "from" -> capture sources
  if (/\bfrom\s+\w*$/.test(trimmed)) {
    const word = context.matchBefore(/\w*/)
    if (word) {
      return {
        from: word.from,
        options: [
          { label: 'body', type: 'variable', detail: 'response body path', apply: 'body.' },
          { label: 'header', type: 'variable', detail: 'response header', apply: 'header ' },
          { label: 'status', type: 'variable', detail: 'HTTP status code' },
          { label: 'duration', type: 'variable', detail: 'response time' },
        ],
        validFor: /^\w*/,
      }
    }
  }

  // After variable name -> "from" keyword
  if (/^\w+\s+\w*$/.test(trimmed) && !/\bfrom\b/.test(trimmed)) {
    const word = context.matchBefore(/\w*/)
    if (word) {
      return {
        from: word.from,
        options: [{ label: 'from', type: 'keyword', detail: 'capture source', apply: 'from ' }],
        validFor: /^\w*/,
      }
    }
  }

  return null
}

function dbCompletions(context: CompletionContext, beforeCursor: string): CompletionResult | null {
  const trimmed = beforeCursor.trimStart()

  // Keywords at start of line
  if (/^(q|qu|que|quer|query|e|ex|exp|expe|expec|expect)$/.test(trimmed)) {
    return {
      from: context.pos - trimmed.length,
      options: [
        { label: 'query', type: 'keyword', detail: 'SQL query', apply: 'query ' },
        { label: 'expect', type: 'keyword', detail: 'DB assertion', apply: 'expect ' },
      ],
      validFor: /^\w*/,
    }
  }

  // After "expect column " -> operators
  const opMatch = trimmed.match(/^expect\s+\w+\s+(!?\w*)$/)
  if (opMatch) {
    const prefix = opMatch[1]
    return {
      from: context.pos - prefix.length,
      options: ASSERTION_OPERATORS.filter((o) =>
        ['==', '!=', '>', '>=', '<', '<=', 'contains', 'exists', '!exists'].includes(o.label),
      ),
      validFor: /^!?\w*/,
    }
  }

  if (context.explicit) {
    return {
      from: context.pos,
      options: [
        { label: 'query', type: 'keyword', apply: 'query ' },
        { label: 'expect', type: 'keyword', apply: 'expect ' },
      ],
    }
  }

  return null
}
