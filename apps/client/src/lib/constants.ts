export const HTTP_METHODS = ['GET', 'POST', 'PUT', 'DELETE', 'PATCH', 'HEAD', 'OPTIONS'] as const
export type HttpMethod = (typeof HTTP_METHODS)[number]

export const METHOD_COLORS: Record<HttpMethod, string> = {
  GET: 'text-http-get',
  POST: 'text-http-post',
  PUT: 'text-http-put',
  DELETE: 'text-http-delete',
  PATCH: 'text-http-patch',
  HEAD: 'text-nord-7',
  OPTIONS: 'text-nord-9',
}

export const METHOD_BG_COLORS: Record<HttpMethod, string> = {
  GET: 'bg-http-get/15',
  POST: 'bg-http-post/15',
  PUT: 'bg-http-put/15',
  DELETE: 'bg-http-delete/15',
  PATCH: 'bg-http-patch/15',
  HEAD: 'bg-nord-7/15',
  OPTIONS: 'bg-nord-9/15',
}

export function statusCodeColor(code: number): string {
  if (code >= 200 && code < 300) return 'text-status-2xx'
  if (code >= 300 && code < 400) return 'text-status-3xx'
  if (code >= 400 && code < 500) return 'text-status-4xx'
  if (code >= 500) return 'text-status-5xx'
  return 'text-muted-foreground'
}

export function statusCodeBgColor(code: number): string {
  if (code >= 200 && code < 300) return 'bg-status-2xx/15'
  if (code >= 300 && code < 400) return 'bg-status-3xx/15'
  if (code >= 400 && code < 500) return 'bg-status-4xx/15'
  if (code >= 500) return 'bg-status-5xx/15'
  return 'bg-muted'
}

export const STATUS_TEXT: Record<number, string> = {
  200: 'OK',
  201: 'Created',
  204: 'No Content',
  301: 'Moved Permanently',
  302: 'Found',
  304: 'Not Modified',
  400: 'Bad Request',
  401: 'Unauthorized',
  403: 'Forbidden',
  404: 'Not Found',
  405: 'Method Not Allowed',
  409: 'Conflict',
  422: 'Unprocessable Entity',
  429: 'Too Many Requests',
  500: 'Internal Server Error',
  502: 'Bad Gateway',
  503: 'Service Unavailable',
  504: 'Gateway Timeout',
}
