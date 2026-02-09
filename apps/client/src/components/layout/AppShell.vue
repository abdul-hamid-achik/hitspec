<script setup lang="ts">
import { onMounted } from 'vue'
import { RouterView } from 'vue-router'
import Sidebar from './Sidebar.vue'
import TopBar from './TopBar.vue'
import StatusBar from './StatusBar.vue'
import { useCollectionStore } from '@/stores/collection'
import { useEnvironmentStore } from '@/stores/environment'
import { useSettingsStore } from '@/stores/settings'
import { ws } from '@/api/websocket'

const collection = useCollectionStore()
const environment = useEnvironmentStore()
const settings = useSettingsStore()

onMounted(() => {
  ws.connect()
  collection.loadFiles()
  environment.loadEnvironments()
  settings.loadConfig()
})
</script>

<template>
  <div class="flex h-screen flex-col overflow-hidden bg-background text-foreground">
    <TopBar />
    <div class="flex flex-1 overflow-hidden">
      <Sidebar :width="settings.sidebarWidth" />
      <main class="flex-1 overflow-auto">
        <slot />
        <RouterView />
      </main>
    </div>
    <StatusBar />
  </div>
</template>
