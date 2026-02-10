<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { Layers, Check } from 'lucide-vue-next'
import { getStressProfiles } from '@/api/endpoints/profiles'
import type { StressProfile } from '@/types/api'

const emit = defineEmits<{
  (e: 'select', profile: StressProfile): void
}>()

const profiles = ref<StressProfile[]>([])
const selectedName = ref<string | null>(null)
const loading = ref(false)

async function loadProfiles() {
  loading.value = true
  try {
    profiles.value = await getStressProfiles()
  } catch {
    // Profiles are optional; silently handle failure
  } finally {
    loading.value = false
  }
}

function selectProfile(profile: StressProfile) {
  selectedName.value = profile.name
  emit('select', profile)
}

onMounted(loadProfiles)
</script>

<template>
  <div class="rounded-lg border border-border bg-background p-4">
    <div class="mb-3 flex items-center gap-2">
      <Layers :size="14" class="text-muted-foreground" />
      <h3 class="text-sm font-medium text-foreground">Profiles</h3>
    </div>

    <div v-if="loading" class="text-xs text-muted-foreground">Loading profiles...</div>

    <div v-else-if="profiles.length === 0" class="text-xs text-muted-foreground">
      No profiles defined. Add profiles to hitspec.yaml under stress.profiles.
    </div>

    <div v-else class="grid gap-2">
      <button
        v-for="profile in profiles"
        :key="profile.name"
        class="flex items-start gap-3 rounded-md border px-3 py-2 text-left transition-colors"
        :class="selectedName === profile.name
          ? 'border-accent bg-accent/10'
          : 'border-border hover:border-accent/50 hover:bg-surface-hover'"
        @click="selectProfile(profile)"
      >
        <div class="flex-1">
          <div class="flex items-center gap-2">
            <span class="text-sm font-medium text-foreground">{{ profile.name }}</span>
            <Check v-if="selectedName === profile.name" :size="14" class="text-accent" />
          </div>
          <div class="mt-1 flex flex-wrap gap-x-3 gap-y-1 text-xs text-muted-foreground">
            <span v-if="profile.duration">{{ profile.duration }}</span>
            <span v-if="profile.rate">{{ profile.rate }} rps</span>
            <span v-if="profile.vus">{{ profile.vus }} VUs</span>
            <span v-if="profile.rampUp">ramp {{ profile.rampUp }}</span>
          </div>
          <div v-if="profile.thresholds" class="mt-1 flex flex-wrap gap-2">
            <span
              v-for="(value, key) in profile.thresholds"
              :key="key"
              class="rounded bg-surface px-1.5 py-0.5 text-[10px] text-muted-foreground"
            >
              {{ key }}: {{ value }}
            </span>
          </div>
        </div>
      </button>
    </div>
  </div>
</template>
