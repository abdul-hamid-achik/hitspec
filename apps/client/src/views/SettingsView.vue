<script setup lang="ts">
import AppShell from '@/components/layout/AppShell.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import { useSettingsStore } from '@/stores/settings'
import { onMounted } from 'vue'

const settings = useSettingsStore()

onMounted(() => {
  settings.loadConfig()
  settings.loadSystemInfo()
})
</script>

<template>
  <AppShell>
    <div class="mx-auto max-w-2xl p-6">
      <h1 class="mb-6 text-lg font-semibold text-foreground">Settings</h1>

      <LoadingSpinner v-if="settings.loading" />
      <div v-else-if="settings.config" class="space-y-6">
        <section>
          <h2 class="mb-3 text-sm font-medium text-foreground">Request Defaults</h2>
          <div class="space-y-3 rounded border border-border bg-surface p-4">
            <label class="flex items-center justify-between">
              <span class="text-sm text-muted-foreground">Timeout (ms)</span>
              <input
                type="number"
                :value="settings.config.timeout"
                class="w-32 rounded border border-border bg-background px-2 py-1 text-right text-sm text-foreground"
                @change="settings.saveConfig({ timeout: Number(($event.target as HTMLInputElement).value) })"
              />
            </label>
            <label class="flex items-center justify-between">
              <span class="text-sm text-muted-foreground">Follow Redirects</span>
              <input
                type="checkbox"
                :checked="settings.config.followRedirects"
                class="accent-accent"
                @change="settings.saveConfig({ followRedirects: ($event.target as HTMLInputElement).checked })"
              />
            </label>
            <label class="flex items-center justify-between">
              <span class="text-sm text-muted-foreground">Max Redirects</span>
              <input
                type="number"
                :value="settings.config.maxRedirects"
                class="w-32 rounded border border-border bg-background px-2 py-1 text-right text-sm text-foreground"
                @change="settings.saveConfig({ maxRedirects: Number(($event.target as HTMLInputElement).value) })"
              />
            </label>
            <label class="flex items-center justify-between">
              <span class="text-sm text-muted-foreground">Skip TLS Verification</span>
              <input
                type="checkbox"
                :checked="settings.config.insecure"
                class="accent-accent"
                @change="settings.saveConfig({ insecure: ($event.target as HTMLInputElement).checked })"
              />
            </label>
            <label class="flex items-center justify-between">
              <span class="text-sm text-muted-foreground">Verbose Output</span>
              <input
                type="checkbox"
                :checked="settings.config.verbose"
                class="accent-accent"
                @change="settings.saveConfig({ verbose: ($event.target as HTMLInputElement).checked })"
              />
            </label>
          </div>
        </section>

        <section v-if="settings.systemInfo">
          <h2 class="mb-3 text-sm font-medium text-foreground">System Info</h2>
          <div class="space-y-2 rounded border border-border bg-surface p-4">
            <div class="flex justify-between text-sm">
              <span class="text-muted-foreground">Version</span>
              <span class="font-mono text-foreground">{{ settings.systemInfo.version }}</span>
            </div>
            <div class="flex justify-between text-sm">
              <span class="text-muted-foreground">Go Version</span>
              <span class="font-mono text-foreground">{{ settings.systemInfo.goVersion }}</span>
            </div>
            <div class="flex justify-between text-sm">
              <span class="text-muted-foreground">Platform</span>
              <span class="font-mono text-foreground">{{ settings.systemInfo.platform }}</span>
            </div>
            <div class="flex justify-between text-sm">
              <span class="text-muted-foreground">Work Directory</span>
              <span class="truncate font-mono text-foreground">{{ settings.systemInfo.workDir }}</span>
            </div>
          </div>
        </section>
      </div>
    </div>
  </AppShell>
</template>
