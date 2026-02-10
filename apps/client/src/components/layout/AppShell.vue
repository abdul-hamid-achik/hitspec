<script lang="ts">
let appInitialized = false
</script>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import Sidebar from './Sidebar.vue'
import TopBar from './TopBar.vue'
import StatusBar from './StatusBar.vue'
import CommandPalette from '@/components/command/CommandPalette.vue'
import KeyboardShortcuts from '@/components/command/KeyboardShortcuts.vue'
import { useCollectionStore } from '@/stores/collection'
import { useEnvironmentStore } from '@/stores/environment'
import { useSettingsStore } from '@/stores/settings'
import { useRequestStore } from '@/stores/request'
import { useKeyboard } from '@/composables/useKeyboard'
import { ws } from '@/api/websocket'

const router = useRouter()
const collection = useCollectionStore()
const environment = useEnvironmentStore()
const settings = useSettingsStore()
const requestStore = useRequestStore()

const commandPaletteOpen = ref(false)
const shortcutsOpen = ref(false)
const sidebarCollapsed = ref(false)

onMounted(async () => {
  if (!appInitialized) {
    appInitialized = true
    ws.connect()
    collection.init()
    // Load workspace first so we can seed the active environment from the server
    await collection.loadFiles()
    if (collection.workspaceEnvironment) {
      environment.activeEnvName = collection.workspaceEnvironment
    }
    environment.loadEnvironments()
    settings.loadConfig()
  }
})

function toggleSidebar() {
  sidebarCollapsed.value = !sidebarCollapsed.value
}

useKeyboard({
  'mod+k': () => { commandPaletteOpen.value = true },
  'mod+b': () => { toggleSidebar() },
  'mod+shift+?': () => { shortcutsOpen.value = true },
  'mod+enter': () => {
    if (collection.activeFilePath) {
      requestStore.runFile(collection.activeFilePath, environment.activeEnvName)
    }
  },
  'mod+1': () => router.push('/'),
  'mod+2': () => router.push('/stress'),
  'mod+3': () => router.push('/mock'),
  'mod+4': () => router.push('/contract'),
  'mod+5': () => router.push('/record'),
  'mod+6': () => router.push('/history'),
  'mod+7': () => router.push('/import'),
  'mod+8': () => router.push('/settings'),
})
</script>

<template>
  <div class="flex h-screen flex-col overflow-hidden bg-background text-foreground">
    <TopBar
      @open-command-palette="commandPaletteOpen = true"
      @toggle-sidebar="toggleSidebar"
    />
    <div class="flex flex-1 overflow-hidden">
      <Sidebar
        :width="settings.sidebarWidth"
        :collapsed="sidebarCollapsed"
        @collapse="toggleSidebar"
      />
      <main aria-label="Main content" class="flex-1 overflow-hidden">
        <slot />
      </main>
    </div>
    <StatusBar />

    <CommandPalette
      v-model:open="commandPaletteOpen"
      @show-shortcuts="shortcutsOpen = true; commandPaletteOpen = false"
    />
    <KeyboardShortcuts v-model:open="shortcutsOpen" />
  </div>
</template>
