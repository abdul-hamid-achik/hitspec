<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { Layers, Check, Plus, Pencil, Trash2, X } from 'lucide-vue-next'
import { getStressProfiles, createStressProfile, updateStressProfile, deleteStressProfile } from '@/api/endpoints/profiles'
import type { StressProfile } from '@/types/api'
import { toast } from 'vue-sonner'

const emit = defineEmits<{
  (e: 'select', profile: StressProfile): void
}>()

const profiles = ref<StressProfile[]>([])
const selectedName = ref<string | null>(null)
const loading = ref(false)

// Form state
const showForm = ref(false)
const editingName = ref<string | null>(null)
const formName = ref('')
const formDuration = ref('')
const formRate = ref<number | undefined>()
const formVUs = ref<number | undefined>()
const formRampUp = ref('')
const saving = ref(false)

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

function openCreateForm() {
  editingName.value = null
  formName.value = ''
  formDuration.value = '30s'
  formRate.value = 100
  formVUs.value = 10
  formRampUp.value = ''
  showForm.value = true
}

function openEditForm(profile: StressProfile) {
  editingName.value = profile.name
  formName.value = profile.name
  formDuration.value = profile.duration ?? ''
  formRate.value = profile.rate
  formVUs.value = profile.vus
  formRampUp.value = profile.rampUp ?? ''
  showForm.value = true
}

function closeForm() {
  showForm.value = false
  editingName.value = null
}

async function handleSave() {
  const name = formName.value.trim()
  if (!name) {
    toast.error('Profile name is required')
    return
  }

  saving.value = true
  try {
    const data: StressProfile = {
      name,
      duration: formDuration.value || undefined,
      rate: formRate.value || undefined,
      vus: formVUs.value || undefined,
      rampUp: formRampUp.value || undefined,
    }

    if (editingName.value) {
      await updateStressProfile(editingName.value, data)
      toast.success(`Profile "${name}" updated`)
    } else {
      await createStressProfile(data)
      toast.success(`Profile "${name}" created`)
    }

    closeForm()
    await loadProfiles()
  } catch (e) {
    toast.error(e instanceof Error ? e.message : 'Failed to save profile')
  } finally {
    saving.value = false
  }
}

async function handleDelete(profile: StressProfile) {
  if (!window.confirm(`Delete profile "${profile.name}"?`)) return
  try {
    await deleteStressProfile(profile.name)
    toast.success(`Profile "${profile.name}" deleted`)
    if (selectedName.value === profile.name) {
      selectedName.value = null
    }
    await loadProfiles()
  } catch (e) {
    toast.error(e instanceof Error ? e.message : 'Failed to delete profile')
  }
}

onMounted(loadProfiles)
</script>

<template>
  <div class="rounded-lg border border-border bg-background p-4">
    <div class="mb-3 flex items-center justify-between">
      <div class="flex items-center gap-2">
        <Layers :size="14" class="text-muted-foreground" />
        <h3 class="text-sm font-medium text-foreground">Profiles</h3>
      </div>
      <button
        v-if="!showForm"
        class="flex items-center gap-1 rounded-md border border-border px-2 py-0.5 text-xs text-muted-foreground transition-colors hover:bg-surface-hover hover:text-foreground"
        @click="openCreateForm"
      >
        <Plus :size="12" />
        New
      </button>
    </div>

    <!-- Create/Edit form -->
    <div v-if="showForm" class="mb-3 space-y-2 rounded-md border border-accent/30 bg-accent/5 p-3">
      <div class="flex items-center justify-between">
        <span class="text-xs font-medium text-foreground">{{ editingName ? 'Edit Profile' : 'New Profile' }}</span>
        <button class="text-muted-foreground hover:text-foreground" @click="closeForm">
          <X :size="14" />
        </button>
      </div>
      <div>
        <label class="mb-1 block text-[10px] text-muted-foreground">Name</label>
        <input
          v-model="formName"
          type="text"
          :disabled="!!editingName"
          placeholder="e.g. smoke-test"
          class="w-full rounded-md border border-border bg-input px-2 py-1 text-xs text-foreground focus:outline-none focus:ring-1 focus:ring-ring disabled:opacity-50"
        />
      </div>
      <div class="grid grid-cols-2 gap-2">
        <div>
          <label class="mb-1 block text-[10px] text-muted-foreground">Duration</label>
          <input
            v-model="formDuration"
            type="text"
            placeholder="30s"
            class="w-full rounded-md border border-border bg-input px-2 py-1 text-xs text-foreground focus:outline-none focus:ring-1 focus:ring-ring"
          />
        </div>
        <div>
          <label class="mb-1 block text-[10px] text-muted-foreground">RPS</label>
          <input
            v-model.number="formRate"
            type="number"
            min="1"
            placeholder="100"
            class="w-full rounded-md border border-border bg-input px-2 py-1 text-xs text-foreground focus:outline-none focus:ring-1 focus:ring-ring"
          />
        </div>
        <div>
          <label class="mb-1 block text-[10px] text-muted-foreground">VUs</label>
          <input
            v-model.number="formVUs"
            type="number"
            min="1"
            placeholder="10"
            class="w-full rounded-md border border-border bg-input px-2 py-1 text-xs text-foreground focus:outline-none focus:ring-1 focus:ring-ring"
          />
        </div>
        <div>
          <label class="mb-1 block text-[10px] text-muted-foreground">Ramp Up</label>
          <input
            v-model="formRampUp"
            type="text"
            placeholder="10s"
            class="w-full rounded-md border border-border bg-input px-2 py-1 text-xs text-foreground focus:outline-none focus:ring-1 focus:ring-ring"
          />
        </div>
      </div>
      <button
        class="flex w-full items-center justify-center gap-1.5 rounded-md bg-accent px-3 py-1.5 text-xs font-medium text-accent-foreground hover:bg-accent/80 disabled:opacity-50"
        :disabled="saving"
        @click="handleSave"
      >
        {{ saving ? 'Saving...' : editingName ? 'Update' : 'Create' }}
      </button>
    </div>

    <div v-if="loading" class="text-xs text-muted-foreground">Loading profiles...</div>

    <div v-else-if="profiles.length === 0 && !showForm" class="text-xs text-muted-foreground">
      No profiles defined. Click "New" to create one.
    </div>

    <div v-else class="grid gap-2">
      <button
        v-for="profile in profiles"
        :key="profile.name"
        class="group flex items-start gap-3 rounded-md border px-3 py-2 text-left transition-colors"
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
        <div class="flex shrink-0 gap-1 opacity-0 transition-opacity group-hover:opacity-100">
          <button
            class="rounded p-1 text-muted-foreground hover:bg-surface-hover hover:text-foreground"
            title="Edit profile"
            @click.stop="openEditForm(profile)"
          >
            <Pencil :size="12" />
          </button>
          <button
            class="rounded p-1 text-muted-foreground hover:bg-destructive/10 hover:text-destructive"
            title="Delete profile"
            @click.stop="handleDelete(profile)"
          >
            <Trash2 :size="12" />
          </button>
        </div>
      </button>
    </div>
  </div>
</template>
