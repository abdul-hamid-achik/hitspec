import type { RequestDTO } from '@/types/api'

export function substituteVars(str: string, vars: Record<string, string>): string {
  return str.replace(/\{\{(\w+)\}\}/g, (_, name) => vars[name] ?? `{{${name}}}`)
}

function enabledHeaders(req: RequestDTO): Array<{ key: string; value: string }> {
  return (req.headers ?? []).filter((h) => h.key)
}

function shellEscape(s: string): string {
  return "'" + s.replace(/'/g, "'\\''") + "'"
}

export function toCurl(req: RequestDTO, vars: Record<string, string>): string {
  const url = substituteVars(req.url, vars)
  const parts: string[] = ['curl']

  if (req.method !== 'GET') {
    parts.push(`-X ${req.method}`)
  }

  parts.push(shellEscape(url))

  for (const h of enabledHeaders(req)) {
    const val = substituteVars(h.value, vars)
    parts.push(`-H ${shellEscape(`${h.key}: ${val}`)}`)
  }

  if (req.body?.raw) {
    const body = substituteVars(req.body.raw, vars)
    parts.push(`-d ${shellEscape(body)}`)
  }

  return parts.join(' \\\n  ')
}

export function toFetch(req: RequestDTO, vars: Record<string, string>): string {
  const url = substituteVars(req.url, vars)
  const headers = enabledHeaders(req)
  const hasHeaders = headers.length > 0
  const hasBody = !!req.body?.raw

  if (req.method === 'GET' && !hasHeaders && !hasBody) {
    return `const response = await fetch('${url}');`
  }

  const lines: string[] = []
  lines.push(`const response = await fetch('${url}', {`)
  lines.push(`  method: '${req.method}',`)

  if (hasHeaders) {
    lines.push('  headers: {')
    for (const h of headers) {
      const val = substituteVars(h.value, vars)
      lines.push(`    '${h.key}': '${val}',`)
    }
    lines.push('  },')
  }

  if (hasBody) {
    const body = substituteVars(req.body!.raw!, vars)
    const isJson = headers.some(
      (h) => h.key.toLowerCase() === 'content-type' && h.value.includes('json'),
    )
    if (isJson) {
      lines.push(`  body: JSON.stringify(${body}),`)
    } else {
      lines.push(`  body: '${body.replace(/'/g, "\\'")}',`)
    }
  }

  lines.push('});')
  return lines.join('\n')
}

export function toWget(req: RequestDTO, vars: Record<string, string>): string {
  const url = substituteVars(req.url, vars)
  const parts: string[] = ['wget']

  if (req.method !== 'GET') {
    parts.push(`--method=${req.method}`)
  }

  for (const h of enabledHeaders(req)) {
    const val = substituteVars(h.value, vars)
    parts.push(`--header=${shellEscape(`${h.key}: ${val}`)}`)
  }

  if (req.body?.raw) {
    const body = substituteVars(req.body.raw, vars)
    parts.push(`--body-data=${shellEscape(body)}`)
  }

  parts.push(shellEscape(url))
  parts.push('-O -')

  return parts.join(' \\\n  ')
}

export function toPythonRequests(req: RequestDTO, vars: Record<string, string>): string {
  const url = substituteVars(req.url, vars)
  const headers = enabledHeaders(req)
  const lines: string[] = ['import requests', '']

  const hasHeaders = headers.length > 0
  const hasBody = !!req.body?.raw
  const isJson = headers.some(
    (h) => h.key.toLowerCase() === 'content-type' && h.value.includes('json'),
  )

  if (hasHeaders) {
    lines.push('headers = {')
    for (const h of headers) {
      const val = substituteVars(h.value, vars)
      lines.push(`    '${h.key}': '${val}',`)
    }
    lines.push('}')
    lines.push('')
  }

  if (hasBody && isJson) {
    const body = substituteVars(req.body!.raw!, vars)
    lines.push(`data = ${body}`)
    lines.push('')
  } else if (hasBody) {
    const body = substituteVars(req.body!.raw!, vars)
    lines.push(`data = '${body.replace(/'/g, "\\'")}'`)
    lines.push('')
  }

  const method = req.method.toLowerCase()
  const args: string[] = [`'${url}'`]
  if (hasHeaders) args.push('headers=headers')
  if (hasBody && isJson) args.push('json=data')
  else if (hasBody) args.push('data=data')

  lines.push(`response = requests.${method}(${args.join(', ')})`)
  lines.push('print(response.status_code)')
  lines.push('print(response.text)')

  return lines.join('\n')
}

export function toHTTPie(req: RequestDTO, vars: Record<string, string>): string {
  const url = substituteVars(req.url, vars)
  const headers = enabledHeaders(req)
  const parts: string[] = ['http', req.method, shellEscape(url)]

  for (const h of headers) {
    const val = substituteVars(h.value, vars)
    parts.push(`${h.key}:${shellEscape(val)}`)
  }

  if (req.body?.raw) {
    const body = substituteVars(req.body.raw, vars)
    const isJson = headers.some(
      (h) => h.key.toLowerCase() === 'content-type' && h.value.includes('json'),
    )
    if (isJson) {
      // For JSON bodies, pipe the data via echo
      return `echo ${shellEscape(body)} | ${parts.join(' \\\n  ')}`
    }
    parts.push(`--raw=${shellEscape(body)}`)
  }

  return parts.join(' \\\n  ')
}

export type ExportFormat = 'curl' | 'fetch' | 'wget' | 'python' | 'httpie'

export const exporters: Record<
  ExportFormat,
  (req: RequestDTO, vars: Record<string, string>) => string
> = {
  curl: toCurl,
  fetch: toFetch,
  wget: toWget,
  python: toPythonRequests,
  httpie: toHTTPie,
}

export const formatLabels: Record<ExportFormat, string> = {
  curl: 'cURL',
  fetch: 'Fetch',
  wget: 'wget',
  python: 'Python',
  httpie: 'HTTPie',
}
