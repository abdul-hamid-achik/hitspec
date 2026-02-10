<script setup lang="ts">
import AppShell from '@/components/layout/AppShell.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import { useSettingsStore } from '@/stores/settings'
import { onMounted, ref } from 'vue'
import { Info } from 'lucide-vue-next'

const settings = useSettingsStore()

// Notification settings (stored locally since backend doesn't have a notifications endpoint yet)
const slackWebhook = ref('')
const teamsWebhook = ref('')
const notifyOnFailure = ref(true)
const notifyOnSuccess = ref(false)

// Database assertion settings
const dbType = ref<'postgresql' | 'mysql' | 'sqlite'>('postgresql')
const dbConnectionString = ref('')
const dbEnabled = ref(false)

onMounted(() => {
  settings.loadConfig()
  settings.loadSystemInfo()
})
</script>

<template>
  <AppShell>
    <div class="h-full overflow-auto"><div class="mx-auto max-w-2xl p-6">
      <h1 class="mb-6 text-lg font-semibold text-foreground">Settings</h1>

      <LoadingSpinner v-if="settings.loading" label="Loading configuration..." />
      <div v-else-if="settings.config" class="space-y-6">
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
            <label class="flex items-center justify-between px-4 py-3 transition-colors hover:bg-surface-hover">
              <span class="text-sm text-foreground">Max Redirects</span>
              <input
                type="number"
                :value="settings.config.maxRedirects"
                class="w-28 rounded-md border border-border bg-background px-2.5 py-1 text-right text-sm tabular-nums text-foreground focus:outline-none focus:ring-1 focus:ring-ring"
                @change="settings.saveConfig({ maxRedirects: Number(($event.target as HTMLInputElement).value) })"
              />
            </label>
            <div class="mx-4 border-t border-border/50" />
            <label class="flex cursor-pointer items-center justify-between px-4 py-3 transition-colors hover:bg-surface-hover">
              <div>
                <span class="text-sm text-foreground">Skip TLS Verification</span>
                <span class="ml-2 text-[10px] text-warning">insecure</span>
              </div>
              <div class="relative">
                <input
                  type="checkbox"
                  :checked="settings.config.insecure"
                  class="peer sr-only"
                  @change="settings.saveConfig({ insecure: ($event.target as HTMLInputElement).checked })"
                />
                <div class="h-5 w-9 rounded-full bg-border transition-colors peer-checked:bg-accent" />
                <div class="absolute left-0.5 top-0.5 h-4 w-4 rounded-full bg-foreground transition-transform peer-checked:translate-x-4" />
              </div>
            </label>
            <div class="mx-4 border-t border-border/50" />
            <label class="flex cursor-pointer items-center justify-between px-4 py-3 transition-colors hover:bg-surface-hover last:rounded-b-lg">
              <span class="text-sm text-foreground">Verbose Output</span>
              <div class="relative">
                <input
                  type="checkbox"
                  :checked="settings.config.verbose"
                  class="peer sr-only"
                  @change="settings.saveConfig({ verbose: ($event.target as HTMLInputElement).checked })"
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
            Notification settings are not persisted yet. Configure in hitspec.yaml for CI/CD use.
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
  </AppShell>
</template>
