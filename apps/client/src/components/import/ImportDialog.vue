<script setup lang="ts">
import { ref } from 'vue'
import CurlImport from './CurlImport.vue'
import InsomniaImport from './InsomniaImport.vue'

const activeTab = ref<'curl' | 'insomnia'>('curl')
</script>

<template>
  <div class="space-y-4">
    <div class="flex gap-2">
      <button
        class="rounded-md px-3 py-1.5 text-sm"
        :class="activeTab === 'curl' ? 'bg-accent text-accent-foreground' : 'bg-surface text-muted-foreground hover:text-foreground'"
        @click="activeTab = 'curl'"
      >
        cURL
      </button>
      <button
        class="rounded-md px-3 py-1.5 text-sm"
        :class="activeTab === 'insomnia' ? 'bg-accent text-accent-foreground' : 'bg-surface text-muted-foreground hover:text-foreground'"
        @click="activeTab = 'insomnia'"
      >
        Insomnia
      </button>
    </div>
    <CurlImport v-if="activeTab === 'curl'" />
    <InsomniaImport v-else-if="activeTab === 'insomnia'" />
  </div>
</template>
