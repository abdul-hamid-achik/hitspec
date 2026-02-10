import { ref, watch } from 'vue'
import { defineStore } from 'pinia'

export interface CookieEntry {
  name: string
  value: string
  domain: string
  path: string
  expires: string
  httpOnly: boolean
  secure: boolean
  sameSite: 'Lax' | 'Strict' | 'None'
}

const STORAGE_KEY = 'hitspec-cookies'

function loadFromStorage(): Map<string, CookieEntry[]> {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (!raw) return new Map()
    const obj = JSON.parse(raw) as Record<string, CookieEntry[]>
    return new Map(Object.entries(obj))
  } catch {
    return new Map()
  }
}

function saveToStorage(cookies: Map<string, CookieEntry[]>) {
  const obj: Record<string, CookieEntry[]> = {}
  for (const [domain, entries] of cookies) {
    obj[domain] = entries
  }
  localStorage.setItem(STORAGE_KEY, JSON.stringify(obj))
}

export const useCookieStore = defineStore('cookies', () => {
  const cookies = ref<Map<string, CookieEntry[]>>(loadFromStorage())

  // Persist on every change
  watch(cookies, (val) => saveToStorage(val), { deep: true })

  function addCookie(cookie: CookieEntry) {
    const domain = cookie.domain || 'unknown'
    const existing = cookies.value.get(domain) || []
    // Replace if same name+path already exists
    const idx = existing.findIndex(c => c.name === cookie.name && c.path === cookie.path)
    if (idx >= 0) {
      existing[idx] = cookie
    } else {
      existing.push(cookie)
    }
    cookies.value.set(domain, existing)
    // Trigger reactivity for Map
    cookies.value = new Map(cookies.value)
  }

  function removeCookie(domain: string, name: string, path: string) {
    const existing = cookies.value.get(domain)
    if (!existing) return
    const filtered = existing.filter(c => !(c.name === name && c.path === path))
    if (filtered.length === 0) {
      cookies.value.delete(domain)
    } else {
      cookies.value.set(domain, filtered)
    }
    cookies.value = new Map(cookies.value)
  }

  function clearDomain(domain: string) {
    cookies.value.delete(domain)
    cookies.value = new Map(cookies.value)
  }

  function clearAll() {
    cookies.value = new Map()
  }

  function captureFromHeaders(headers: Record<string, string>, requestDomain?: string) {
    // Look for Set-Cookie headers (case-insensitive)
    for (const [key, value] of Object.entries(headers)) {
      if (key.toLowerCase() !== 'set-cookie') continue
      // May be multiple cookies separated by newlines
      const lines = value.split('\n')
      for (const line of lines) {
        parseCookieHeader(line.trim(), requestDomain)
      }
    }
  }

  function parseCookieHeader(header: string, requestDomain?: string) {
    if (!header) return
    const parts = header.split(';').map(s => s.trim())
    const firstPart = parts[0]
    const eqIdx = firstPart.indexOf('=')
    if (eqIdx < 0) return

    const name = firstPart.slice(0, eqIdx).trim()
    const value = firstPart.slice(eqIdx + 1).trim()

    const cookie: CookieEntry = {
      name,
      value,
      domain: requestDomain || 'unknown',
      path: '/',
      expires: '',
      httpOnly: false,
      secure: false,
      sameSite: 'Lax',
    }

    for (let i = 1; i < parts.length; i++) {
      const attr = parts[i]
      const attrEq = attr.indexOf('=')
      const attrName = (attrEq >= 0 ? attr.slice(0, attrEq) : attr).trim().toLowerCase()
      const attrValue = attrEq >= 0 ? attr.slice(attrEq + 1).trim() : ''

      switch (attrName) {
        case 'domain':
          cookie.domain = attrValue.replace(/^\./, '')
          break
        case 'path':
          cookie.path = attrValue
          break
        case 'expires':
          cookie.expires = attrValue
          break
        case 'max-age': {
          const seconds = parseInt(attrValue, 10)
          if (!isNaN(seconds)) {
            cookie.expires = new Date(Date.now() + seconds * 1000).toISOString()
          }
          break
        }
        case 'httponly':
          cookie.httpOnly = true
          break
        case 'secure':
          cookie.secure = true
          break
        case 'samesite':
          if (['lax', 'strict', 'none'].includes(attrValue.toLowerCase())) {
            cookie.sameSite = (attrValue.charAt(0).toUpperCase() + attrValue.slice(1).toLowerCase()) as CookieEntry['sameSite']
          }
          break
      }
    }

    addCookie(cookie)
  }

  const domainCount = () => cookies.value.size
  const totalCount = () => {
    let count = 0
    for (const entries of cookies.value.values()) {
      count += entries.length
    }
    return count
  }

  return {
    cookies,
    addCookie,
    removeCookie,
    clearDomain,
    clearAll,
    captureFromHeaders,
    domainCount,
    totalCount,
  }
})
