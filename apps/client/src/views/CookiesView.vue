<script setup lang="ts">
import EmptyState from '@/components/common/EmptyState.vue'
import { useCookieStore, type CookieEntry } from '@/stores/cookies'
import {
  Cookie,
  Trash2,
  Plus,
  ChevronDown,
  ChevronRight,
  Search,
  X,
} from 'lucide-vue-next'
import { ref, computed } from 'vue'
import dayjs from 'dayjs'
import relativeTime from 'dayjs/plugin/relativeTime'

dayjs.extend(relativeTime)

const cookieStore = useCookieStore()

const searchQuery = ref('')
const expandedDomains = ref<Set<string>>(new Set())
const expandedCookie = ref<string | null>(null) // "domain:name:path" key
const showAddForm = ref(false)

// Add form state
const newCookie = ref<CookieEntry>({
  name: '',
  value: '',
  domain: '',
  path: '/',
  expires: '',
  httpOnly: false,
  secure: false,
  sameSite: 'Lax',
})

const domains = computed(() => {
  const result: { domain: string; cookies: CookieEntry[] }[] = []
  for (const [domain, entries] of cookieStore.cookies) {
    const q = searchQuery.value.toLowerCase()
    if (q) {
      const filtered = entries.filter(
        c => c.domain.toLowerCase().includes(q) || c.name.toLowerCase().includes(q),
      )
      if (filtered.length > 0) {
        result.push({ domain, cookies: filtered })
      }
    } else {
      result.push({ domain, cookies: entries })
    }
  }
  result.sort((a, b) => a.domain.localeCompare(b.domain))
  return result
})

function toggleDomain(domain: string) {
  if (expandedDomains.value.has(domain)) {
    expandedDomains.value.delete(domain)
  } else {
    expandedDomains.value.add(domain)
  }
  expandedDomains.value = new Set(expandedDomains.value)
}

function cookieKey(cookie: CookieEntry): string {
  return `${cookie.domain}:${cookie.name}:${cookie.path}`
}

function toggleCookieDetail(cookie: CookieEntry) {
  const key = cookieKey(cookie)
  expandedCookie.value = expandedCookie.value === key ? null : key
}

function truncate(str: string, len: number): string {
  return str.length > len ? str.slice(0, len) + '...' : str
}

function formatExpires(expires: string): string {
  if (!expires) return 'Session'
  const d = dayjs(expires)
  if (!d.isValid()) return expires
  if (d.isBefore(dayjs())) return 'Expired'
  return d.fromNow()
}

function handleAddCookie() {
  if (!newCookie.value.name.trim() || !newCookie.value.domain.trim()) return
  cookieStore.addCookie({ ...newCookie.value })
  // Expand the domain to show the new cookie
  expandedDomains.value.add(newCookie.value.domain)
  expandedDomains.value = new Set(expandedDomains.value)
  // Reset form
  newCookie.value = {
    name: '',
    value: '',
    domain: '',
    path: '/',
    expires: '',
    httpOnly: false,
    secure: false,
    sameSite: 'Lax',
  }
  showAddForm.value = false
}

function confirmClearAll() {
  if (window.confirm('Clear all cookies? This cannot be undone.')) {
    cookieStore.clearAll()
  }
}

function confirmClearDomain(domain: string, event: Event) {
  event.stopPropagation()
  if (window.confirm(`Clear all cookies for ${domain}?`)) {
    cookieStore.clearDomain(domain)
  }
}
</script>

<template>
  <div class="h-full overflow-auto p-6">
    <div class="mb-4 flex items-center justify-between">
      <h1 class="text-lg font-semibold text-foreground">Cookies</h1>
      <div class="flex items-center gap-2">
        <button
          class="flex items-center gap-1.5 rounded-md border border-border px-2.5 py-1 text-xs text-muted-foreground transition-colors hover:bg-accent/10 hover:text-accent"
          @click="showAddForm = !showAddForm"
        >
          <Plus class="h-3 w-3" />
          Add Cookie
        </button>
        <button
          v-if="cookieStore.totalCount() > 0"
          aria-label="Clear all cookies"
          class="flex items-center gap-1.5 rounded-md border border-border px-2.5 py-1 text-xs text-muted-foreground transition-colors hover:border-destructive/50 hover:text-destructive"
          @click="confirmClearAll"
        >
          <Trash2 class="h-3 w-3" />
          Clear All
        </button>
      </div>
    </div>

    <!-- Add cookie form -->
    <div v-if="showAddForm" class="mb-4 rounded-lg border border-border bg-surface p-4">
      <div class="mb-3 flex items-center justify-between">
        <h2 class="text-xs font-semibold uppercase tracking-wider text-muted-foreground/60">Add Cookie</h2>
        <button
          aria-label="Close form"
          class="rounded p-1 text-muted-foreground/60 transition-colors hover:bg-surface-hover hover:text-foreground"
          @click="showAddForm = false"
        >
          <X class="h-3.5 w-3.5" />
        </button>
      </div>
      <div class="grid grid-cols-2 gap-3">
        <div>
          <label class="mb-1 block text-xs text-muted-foreground">Domain <span class="text-destructive">*</span></label>
          <input
            v-model="newCookie.domain"
            type="text"
            placeholder="example.com"
            class="w-full rounded-md border border-border bg-background px-2.5 py-1.5 text-xs text-foreground placeholder:text-muted-foreground/60 focus:outline-none focus:ring-1 focus:ring-ring"
          />
        </div>
        <div>
          <label class="mb-1 block text-xs text-muted-foreground">Name <span class="text-destructive">*</span></label>
          <input
            v-model="newCookie.name"
            type="text"
            placeholder="session_id"
            class="w-full rounded-md border border-border bg-background px-2.5 py-1.5 text-xs text-foreground placeholder:text-muted-foreground/60 focus:outline-none focus:ring-1 focus:ring-ring"
          />
        </div>
        <div class="col-span-2">
          <label class="mb-1 block text-xs text-muted-foreground">Value</label>
          <input
            v-model="newCookie.value"
            type="text"
            placeholder="abc123..."
            class="w-full rounded-md border border-border bg-background px-2.5 py-1.5 font-mono text-xs text-foreground placeholder:text-muted-foreground/60 focus:outline-none focus:ring-1 focus:ring-ring"
          />
        </div>
        <div>
          <label class="mb-1 block text-xs text-muted-foreground">Path</label>
          <input
            v-model="newCookie.path"
            type="text"
            placeholder="/"
            class="w-full rounded-md border border-border bg-background px-2.5 py-1.5 text-xs text-foreground placeholder:text-muted-foreground/60 focus:outline-none focus:ring-1 focus:ring-ring"
          />
        </div>
        <div>
          <label class="mb-1 block text-xs text-muted-foreground">Expires</label>
          <input
            v-model="newCookie.expires"
            type="datetime-local"
            class="w-full rounded-md border border-border bg-background px-2.5 py-1.5 text-xs text-foreground focus:outline-none focus:ring-1 focus:ring-ring"
          />
        </div>
        <div>
          <label class="mb-1 block text-xs text-muted-foreground">SameSite</label>
          <select
            v-model="newCookie.sameSite"
            class="w-full rounded-md border border-border bg-background px-2.5 py-1.5 text-xs text-foreground focus:outline-none focus:ring-1 focus:ring-ring"
          >
            <option value="Lax">Lax</option>
            <option value="Strict">Strict</option>
            <option value="None">None</option>
          </select>
        </div>
        <div class="flex items-end gap-4">
          <label class="flex cursor-pointer items-center gap-1.5 text-xs text-foreground">
            <input v-model="newCookie.httpOnly" type="checkbox" class="rounded border-border accent-accent" />
            HttpOnly
          </label>
          <label class="flex cursor-pointer items-center gap-1.5 text-xs text-foreground">
            <input v-model="newCookie.secure" type="checkbox" class="rounded border-border accent-accent" />
            Secure
          </label>
        </div>
      </div>
      <div class="mt-3 flex justify-end">
        <button
          class="rounded-md bg-accent px-4 py-1.5 text-xs font-medium text-accent-foreground transition-colors hover:bg-accent/80 disabled:opacity-50"
          :disabled="!newCookie.name.trim() || !newCookie.domain.trim()"
          @click="handleAddCookie"
        >
          Add Cookie
        </button>
      </div>
    </div>

    <!-- Search -->
    <div v-if="cookieStore.totalCount() > 0" class="mb-3">
      <div class="relative">
        <Search class="absolute left-2.5 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-muted-foreground/60" />
        <input
          v-model="searchQuery"
          type="text"
          placeholder="Search by domain or cookie name..."
          aria-label="Search cookies"
          class="w-full rounded-md border border-border bg-background py-1.5 pl-8 pr-3 text-xs text-foreground placeholder:text-muted-foreground/60 focus:outline-none focus:ring-1 focus:ring-ring"
        />
      </div>
    </div>

    <!-- Cookie list -->
    <div v-if="cookieStore.totalCount() > 0" class="space-y-2">
      <div v-if="domains.length === 0 && searchQuery" class="p-8 text-center text-sm text-muted-foreground">
        No cookies match your search.
      </div>

      <div v-for="group in domains" :key="group.domain" class="rounded-lg border border-border bg-surface">
        <!-- Domain header -->
        <button
          class="flex w-full items-center gap-3 p-3 text-left transition-colors hover:bg-surface-hover"
          @click="toggleDomain(group.domain)"
        >
          <component
            :is="expandedDomains.has(group.domain) ? ChevronDown : ChevronRight"
            class="h-4 w-4 shrink-0 text-muted-foreground/60"
          />
          <span class="flex-1 truncate font-mono text-xs text-foreground/80">
            {{ group.domain }}
          </span>
          <span class="rounded bg-accent/10 px-1.5 py-0.5 text-[10px] tabular-nums text-accent">
            {{ group.cookies.length }} {{ group.cookies.length === 1 ? 'cookie' : 'cookies' }}
          </span>
          <button
            aria-label="Clear domain cookies"
            class="rounded p-1 text-muted-foreground/50 transition-colors hover:bg-destructive/10 hover:text-destructive"
            title="Clear domain"
            @click="confirmClearDomain(group.domain, $event)"
          >
            <Trash2 class="h-3 w-3" />
          </button>
        </button>

        <!-- Expanded cookies -->
        <div v-if="expandedDomains.has(group.domain)" class="border-t border-border">
          <div class="divide-y divide-border/50">
            <div v-for="cookie in group.cookies" :key="cookieKey(cookie)">
              <!-- Cookie summary row -->
              <button
                class="flex w-full items-center gap-2 px-4 py-2 text-left transition-colors hover:bg-surface-hover"
                @click="toggleCookieDetail(cookie)"
              >
                <component
                  :is="expandedCookie === cookieKey(cookie) ? ChevronDown : ChevronRight"
                  class="h-3 w-3 shrink-0 text-muted-foreground/50"
                />
                <span class="font-mono text-xs font-medium text-foreground/80">{{ cookie.name }}</span>
                <span class="flex-1 truncate font-mono text-[11px] text-muted-foreground/50">
                  {{ truncate(cookie.value, 40) }}
                </span>
                <span class="text-[11px] text-muted-foreground/60">{{ cookie.path }}</span>
                <span class="text-[11px] text-muted-foreground/60">{{ formatExpires(cookie.expires) }}</span>
                <button
                  aria-label="Delete cookie"
                  class="rounded p-1 text-muted-foreground/50 transition-colors hover:bg-destructive/10 hover:text-destructive"
                  title="Delete cookie"
                  @click.stop="cookieStore.removeCookie(cookie.domain, cookie.name, cookie.path)"
                >
                  <Trash2 class="h-3 w-3" />
                </button>
              </button>

              <!-- Cookie detail view -->
              <div
                v-if="expandedCookie === cookieKey(cookie)"
                class="mx-4 mb-2 rounded border border-border/50 bg-background"
              >
                <div
                  v-for="(detail, i) in [
                    { label: 'Name', value: cookie.name },
                    { label: 'Value', value: cookie.value },
                    { label: 'Domain', value: cookie.domain },
                    { label: 'Path', value: cookie.path },
                    { label: 'Expires', value: cookie.expires ? formatExpires(cookie.expires) : 'Session' },
                    { label: 'HttpOnly', value: cookie.httpOnly ? 'Yes' : 'No' },
                    { label: 'Secure', value: cookie.secure ? 'Yes' : 'No' },
                    { label: 'SameSite', value: cookie.sameSite },
                  ]"
                  :key="detail.label"
                  class="flex items-center gap-2 border-b border-border/30 px-3 py-1.5 text-[11px] last:border-0"
                >
                  <span class="w-16 shrink-0 text-muted-foreground/60">{{ detail.label }}</span>
                  <span
                    :class="[
                      'flex-1 break-all',
                      i === 1 ? 'font-mono text-foreground/70' : 'text-foreground/60',
                    ]"
                  >{{ detail.value }}</span>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <EmptyState
      v-else
      :icon="Cookie"
      title="No cookies yet"
      description="Add cookies manually or they will be captured from response headers"
    />
  </div>
</template>
