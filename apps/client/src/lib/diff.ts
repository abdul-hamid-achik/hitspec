export interface DiffLine {
  type: 'add' | 'remove' | 'equal'
  content: string
  oldLineNum?: number
  newLineNum?: number
}

/**
 * Compute a line-by-line diff between two strings using the Myers diff algorithm.
 * If both inputs are valid JSON, they are pretty-printed before diffing.
 */
export function computeDiff(oldText: string, newText: string): DiffLine[] {
  const a = toLines(oldText)
  const b = toLines(newText)
  const edits = myersDiff(a, b)
  return numberLines(edits)
}

function tryFormatJson(text: string): string | null {
  try {
    return JSON.stringify(JSON.parse(text), null, 2)
  } catch {
    return null
  }
}

function toLines(text: string): string[] {
  const formatted = tryFormatJson(text)
  return (formatted ?? text).split('\n')
}

function numberLines(edits: Array<{ type: DiffLine['type']; content: string }>): DiffLine[] {
  let oldNum = 1
  let newNum = 1
  return edits.map((e) => {
    const line: DiffLine = { type: e.type, content: e.content }
    if (e.type === 'equal') {
      line.oldLineNum = oldNum++
      line.newLineNum = newNum++
    } else if (e.type === 'remove') {
      line.oldLineNum = oldNum++
    } else {
      line.newLineNum = newNum++
    }
    return line
  })
}

/**
 * Myers diff algorithm (O((N+M)D) time).
 * Returns a sequence of add/remove/equal edits.
 */
function myersDiff(
  a: string[],
  b: string[],
): Array<{ type: DiffLine['type']; content: string }> {
  const n = a.length
  const m = b.length
  const max = n + m

  // Shortcut: if both are empty
  if (n === 0 && m === 0) return []
  if (n === 0) return b.map((l) => ({ type: 'add' as const, content: l }))
  if (m === 0) return a.map((l) => ({ type: 'remove' as const, content: l }))

  // v[k] = furthest reaching x on diagonal k
  // We offset k by max so index is always >= 0
  const size = 2 * max + 1
  const v = new Int32Array(size)
  v.fill(-1)
  const trace: Int32Array[] = []

  v[max + 1] = 0

  for (let d = 0; d <= max; d++) {
    const snapshot = new Int32Array(v)
    trace.push(snapshot)

    for (let k = -d; k <= d; k += 2) {
      const idx = k + max
      let x: number
      if (k === -d || (k !== d && v[idx - 1] < v[idx + 1])) {
        x = v[idx + 1] // move down
      } else {
        x = v[idx - 1] + 1 // move right
      }
      let y = x - k
      // follow diagonal (matches)
      while (x < n && y < m && a[x] === b[y]) {
        x++
        y++
      }
      v[idx] = x
      if (x >= n && y >= m) {
        // Trace back to build the edit script
        return buildEditScript(trace, a, b, d, max)
      }
    }
  }

  // Fallback (should not reach here)
  return [
    ...a.map((l) => ({ type: 'remove' as const, content: l })),
    ...b.map((l) => ({ type: 'add' as const, content: l })),
  ]
}

function buildEditScript(
  trace: Int32Array[],
  a: string[],
  b: string[],
  d: number,
  max: number,
): Array<{ type: DiffLine['type']; content: string }> {
  const edits: Array<{ type: DiffLine['type']; content: string }> = []
  let x = a.length
  let y = b.length

  for (let step = d; step > 0; step--) {
    const v = trace[step - 1]
    const k = x - y
    const idx = k + max

    let prevK: number
    if (k === -step || (k !== step && v[idx - 1] < v[idx + 1])) {
      prevK = k + 1 // came from above (insertion)
    } else {
      prevK = k - 1 // came from left (deletion)
    }

    const prevX = v[prevK + max]
    const prevY = prevX - prevK

    // Diagonal moves (equal lines)
    while (x > prevX && y > prevY) {
      x--
      y--
      edits.push({ type: 'equal', content: a[x] })
    }

    if (step > 0) {
      if (x === prevX) {
        // insertion
        y--
        edits.push({ type: 'add', content: b[y] })
      } else {
        // deletion
        x--
        edits.push({ type: 'remove', content: a[x] })
      }
    }
  }

  // Remaining diagonal from (0,0) to (prevX, prevY)
  while (x > 0 && y > 0) {
    x--
    y--
    edits.push({ type: 'equal', content: a[x] })
  }

  edits.reverse()
  return edits
}

/** Format a diff as a unified diff string for clipboard copy */
export function formatUnifiedDiff(
  lines: DiffLine[],
  leftLabel: string,
  rightLabel: string,
): string {
  const out: string[] = []
  out.push(`--- ${leftLabel}`)
  out.push(`+++ ${rightLabel}`)
  for (const line of lines) {
    if (line.type === 'remove') {
      out.push(`-${line.content}`)
    } else if (line.type === 'add') {
      out.push(`+${line.content}`)
    } else {
      out.push(` ${line.content}`)
    }
  }
  return out.join('\n')
}
