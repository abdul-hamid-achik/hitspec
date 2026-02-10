<script setup lang="ts">
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import { useSettingsStore } from '@/stores/settings'
import { useThemeStore, type ThemeMode } from '@/stores/theme'
import { AlertCircle, Sun, Moon, Monitor } from 'lucide-vue-next'
import { onMounted, ref, watch } from 'vue'

const settings = useSettingsStore()
const theme = useThemeStore()

const themeOptions: { value: ThemeMode; label: string; icon: typeof Sun }[] = [
  { value: 'light', label: 'Light', icon: Sun },
  { value: 'dark', label: 'Dark', icon: Moon },
  { value: 'system', label: 'System', icon: Monitor },
]

// Persist notification and DB settings to localStorage
function loadLocal<T>(key: string, fallback: T): T {
  try {
    const v = localStorage.getItem(`hitspec-${key}`)
    return v !== null ? JSON.parse(v) as T : fallback
  } catch { return fallback }
}
function saveLocal(key: string, value: unknown) {
  localStorage.setItem(`hitspec-${key}`, JSON.stringify(value))
}

const slackWebhook = ref(loadLocal('slack-webhook', ''))
const teamsWebhook = ref(loadLocal('teams-webhook', ''))
const notifyOnFailure = ref(loadLocal('notify-failure', true))
const notifyOnSuccess = ref(loadLocal('notify-success', false))
const dbType = ref<'postgresql' | 'mysql' | 'sqlite'>(loadLocal('db-type', 'postgresql'))
const dbConnectionString = ref(loadLocal('db-connection', ''))
const dbEnabled = ref(loadLocal('db-enabled', false))

// Watch and persist changes
watch(slackWebhook, v => saveLocal('slack-webhook', v))
watch(teamsWebhook, v => saveLocal('teams-webhook', v))
watch(notifyOnFailure, v => saveLocal('notify-failure', v))
watch(notifyOnSuccess, v => saveLocal('notify-success', v))
watch(dbType, v => saveLocal('db-type', v))
watch(dbConnectionString, v => saveLocal('db-connection', v))
watch(dbEnabled, v => saveLocal('db-enabled', v))

onMounted(() => {
  settings.loadConfig()
  settings.loadSystemInfo()
})
</script>

<template>
  <div class="h-full overflow-auto"><div class="mx-auto max-w-2xl p-6">
      <h1 class="mb-6 text-lg font-semibold text-foreground">Settings</h1>

      <LoadingSpinner v-if="settings.loading" label="Loading configuration..." />
      <div v-else-if="settings.error && !settings.config" class="flex flex-col items-center gap-3 p-8">
        <div class="rounded-xl bg-destructive/10 p-3">
          <AlertCircle class="h-8 w-8 text-destructive/60" />
        </div>
        <p class="text-sm text-destructive">{{ settings.error }}</p>
        <button
          class="rounded-md border border-border px-3 py-1.5 text-xs text-muted-foreground transition-colors hover:bg-surface-hover hover:text-foreground"
          @click="settings.loadConfig(true)"
        >
          Retry
        </button>
      </div>
      <div v-else-if="settings.config" class="space-y-6">
        <!-- Appearance -->
        <section>
          <h2 class="mb-3 text-xs font-semibold uppercase tracking-wider text-muted-foreground/60">Appearance</h2>
          <div class="rounded-lg border border-border bg-surface">
            <div class="flex items-center justify-between px-4 py-3">
              <div>
                <span class="text-sm text-foreground">Theme</span>
                <p class="text-xs text-muted-foreground/50">Select your preferred color scheme</p>
              </div>
              <div class="flex gap-1 rounded-lg border border-border bg-background p-0.5">
                <button
                  v-for="opt in themeOptions"
                  :key="opt.value"
                  :aria-label="`Switch to ${opt.label} theme`"
                  class="flex items-center gap-1.5 rounded-md px-2.5 py-1 text-xs transition-colors"
                  :class="theme.mode === opt.value
                    ? 'bg-accent text-accent-foreground'
                    : 'text-muted-foreground hover:text-foreground'"
                  @click="theme.setMode(opt.value)"
                >
                  <component :is="opt.icon" class="h-3.5 w-3.5" />
                  {{ opt.label }}
                </button>
              </div>
            </div>
          </div>
        </section>

        <!-- Request Defaults -->
        <section>
          <h2 class="mb-3 text-xs font-semibold uppercase tracking-wider text-muted-foreground/60">Request Defaults</h2>
          <div class="space-y-1 rounded-lg border border-border bg-surface">
            <label class="flex items-center justify-between px-4 py-3 transition-colors hover:bg-surface-hover first:rounded-t-lg">
              <div>
                <span class="text-sm text-foreground">Timeout</span>
                <span class="ml-2 text-xs text-muted-foreground/50">ms</span>
              </div>
              <input
                type="number"
                :value="settings.config.timeout"
                class="w-28 rounded-md border border-border bg-background px-2.5 py-1 text-right text-sm tabular-nums text-foreground focus:outline-none focus:ring-1 focus:ring-ring"
                @change="settings.saveConfig({ timeout: Number(($event.target as HTMLInputElement).value) })"
              />
            </label>
            <div class="mx-4 border-t border-border/50" />
            <label class="flex cursor-pointer items-center justify-between px-4 py-3 transition-colors hover:bg-surface-hover">
              <span class="text-sm text-foreground">Follow Redirects</span>
              <div class="relative">
                <input
                  type="checkbox"
                  :checked="settings.config.followRedirects"
                  class="peer sr-only"
                  @change="settings.saveConfig({ followRedirects: ($event.target as HTMLInputElement).checked })"
                />
                <div class="h-5 w-9 rounded-full bg-border transition-colors peer-checked:bg-accent" />
                <div class="absolute left-0.5 top-0.5 h-4 w-4 rounded-full bg-foreground transition-transform peer-checked:translate-x-4" />
              </div>
            </label>
            <div class="mx-4 border-t border-border/50" />
            <label class="flex cursor-pointer items-center justify-between px-4 py-3 transition-colors hover:bg-surface-hover">
              <span class="text-sm text-foreground">Validate SSL Certificates</span>
              <div class="relative">
                <input
                  type="checkbox"
                  :checked="settings.config.validateSSL"
                  class="peer sr-only"
                  @change="settings.saveConfig({ validateSSL: ($event.target as HTMLInputElement).checked })"
                />
                <div class="h-5 w-9 rounded-full bg-border transition-colors peer-checked:bg-accent" />
                <div class="absolute left-0.5 top-0.5 h-4 w-4 rounded-full bg-foreground transition-transform peer-checked:translate-x-4" />
              </div>
            </label>
            <div class="mx-4 border-t border-border/50" />
            <label class="flex cursor-pointer items-center justify-between px-4 py-3 transition-colors hover:bg-surface-hover last:rounded-b-lg">
              <span class="text-sm text-foreground">Parallel Execution</span>
              <div class="relative">
                <input
                  type="checkbox"
                  :checked="settings.config.parallel"
                  class="peer sr-only"
                  @change="settings.saveConfig({ parallel: ($event.target as HTMLInputElement).checked })"
                />
                <div class="h-5 w-9 rounded-full bg-border transition-colors peer-checked:bg-accent" />
                <div class="absolute left-0.5 top-0.5 h-4 w-4 rounded-full bg-foreground transition-transform peer-checked:translate-x-4" />
              </div>
            </label>
          </div>
        </section>

        <!-- Notifications -->
        <section>
          <h2 class="mb-3 text-xs font-semibold uppercase tracking-wider text-muted-foreground/60">Notifications</h2>
          <div class="space-y-1 rounded-lg border border-border bg-surface">
            <div class="px-4 py-3">
              <label class="mb-1 block text-sm text-foreground">Slack Webhook URL</label>
              <input
                v-model="slackWebhook"
                type="text"
                placeholder="https://hooks.slack.com/services/..."
                class="w-full rounded-md border border-border bg-background px-2.5 py-1.5 text-sm text-foreground focus:outline-none focus:ring-1 focus:ring-ring"
              />
            </div>
            <div class="mx-4 border-t border-border/50" />
            <div class="px-4 py-3">
              <label class="mb-1 block text-sm text-foreground">Microsoft Teams Webhook URL</label>
              <input
                v-model="teamsWebhook"
                type="text"
                placeholder="https://outlook.office.com/webhook/..."
                class="w-full rounded-md border border-border bg-background px-2.5 py-1.5 text-sm text-foreground focus:outline-none focus:ring-1 focus:ring-ring"
              />
            </div>
            <div class="mx-4 border-t border-border/50" />
            <label class="flex cursor-pointer items-center justify-between px-4 py-3 transition-colors hover:bg-surface-hover">
              <span class="text-sm text-foreground">Notify on Failure</span>
              <div class="relative">
                <input
                  v-model="notifyOnFailure"
                  type="checkbox"
                  class="peer sr-only"
                />
                <div class="h-5 w-9 rounded-full bg-border transition-colors peer-checked:bg-accent" />
                <div class="absolute left-0.5 top-0.5 h-4 w-4 rounded-full bg-foreground transition-transform peer-checked:translate-x-4" />
              </div>
            </label>
            <div class="mx-4 border-t border-border/50" />
            <label class="flex cursor-pointer items-center justify-between px-4 py-3 transition-colors hover:bg-surface-hover">
              <span class="text-sm text-foreground">Notify on Success</span>
              <div class="relative">
                <input
                  v-model="notifyOnSuccess"
                  type="checkbox"
                  class="peer sr-only"
                />
                <div class="h-5 w-9 rounded-full bg-border transition-colors peer-checked:bg-accent" />
                <div class="absolute left-0.5 top-0.5 h-4 w-4 rounded-full bg-foreground transition-transform peer-checked:translate-x-4" />
              </div>
            </label>
          </div>
          <p class="mt-2 text-[10px] text-muted-foreground/50">
            Saved locally in your browser. Configure in hitspec.yaml for CI/CD use.
          </p>
        </section>

        <!-- Database Assertions -->
        <section>
          <h2 class="mb-3 text-xs font-semibold uppercase tracking-wider text-muted-foreground/60">Database Assertions</h2>
          <div class="space-y-1 rounded-lg border border-border bg-surface">
            <label class="flex cursor-pointer items-center justify-between px-4 py-3 transition-colors hover:bg-surface-hover first:rounded-t-lg">
              <span class="text-sm text-foreground">Enable DB Assertions</span>
              <div class="relative">
                <input
                  v-model="dbEnabled"
                  type="checkbox"
                  class="peer sr-only"
                />
                <div class="h-5 w-9 rounded-full bg-border transition-colors peer-checked:bg-accent" />
                <div class="absolute left-0.5 top-0.5 h-4 w-4 rounded-full bg-foreground transition-transform peer-checked:translate-x-4" />
              </div>
            </label>
            <template v-if="dbEnabled">
              <div class="mx-4 border-t border-border/50" />
              <div class="px-4 py-3">
                <label class="mb-1 block text-sm text-foreground">Database Type</label>
                <select
                  v-model="dbType"
                  class="w-full rounded-md border border-border bg-background px-2.5 py-1.5 text-sm text-foreground focus:outline-none focus:ring-1 focus:ring-ring"
                >
                  <option value="postgresql">PostgreSQL</option>
                  <option value="mysql">MySQL</option>
                  <option value="sqlite">SQLite</option>
                </select>
              </div>
              <div class="mx-4 border-t border-border/50" />
              <div class="px-4 py-3">
                <label class="mb-1 block text-sm text-foreground">Connection String</label>
                <input
                  v-model="dbConnectionString"
                  type="text"
                  :placeholder="dbType === 'sqlite' ? './test.db' : dbType === 'postgresql' ? 'postgres://user:pass@localhost:5432/db' : 'user:pass@tcp(localhost:3306)/db'"
                  class="w-full rounded-md border border-border bg-background px-2.5 py-1.5 font-mono text-sm text-foreground focus:outline-none focus:ring-1 focus:ring-ring"
                />
              </div>
            </template>
          </div>
          <p class="mt-2 text-[10px] text-muted-foreground/50">
            Requires --allow-db flag when starting the server. Use >>>db blocks in .http files.
          </p>
        </section>

        <!-- System Info -->
        <section v-if="settings.systemInfo">
          <h2 class="mb-3 text-xs font-semibold uppercase tracking-wider text-muted-foreground/60">System Info</h2>
          <div class="rounded-lg border border-border bg-surface">
            <div v-for="(item, i) in [
              { label: 'Version', value: settings.systemInfo.version },
              { label: 'Go Version', value: settings.systemInfo.goVersion },
              { label: 'Platform', value: `${settings.systemInfo.os}/${settings.systemInfo.arch}` },
              { label: 'Build Time', value: settings.systemInfo.buildTime },
            ]" :key="item.label">
              <div v-if="i > 0" class="mx-4 border-t border-border/50" />
              <div class="flex items-center justify-between px-4 py-2.5 text-sm">
                <span class="text-muted-foreground/70">{{ item.label }}</span>
                <span class="max-w-[300px] truncate font-mono text-xs text-foreground/80">{{ item.value }}</span>
              </div>
            </div>
          </div>
        </section>
      </div>
  </div></div>
</template>
