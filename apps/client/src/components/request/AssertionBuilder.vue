<script setup lang="ts">
import { ref, computed } from 'vue'
import { Plus, Trash2, Copy, ChevronDown } from 'lucide-vue-next'
import { ASSERTION_OPERATORS, ASSERTION_SUBJECTS, type AssertionDTO } from '@/types/api'

const { assertions } = defineProps<{ assertions: AssertionDTO[] }>()
const emit = defineEmits<{
  (e: 'add', assertion: AssertionDTO): void
  (e: 'remove', index: number): void
  (e: 'copy', text: string): void
}>()

const subjectType = ref<string>('status')
const subjectExtra = ref('')
const operator = ref('==')
const expected = ref('')
const showBuilder = ref(false)

const needsExtra = computed(() => {
  return subjectType.value === 'header' || subjectType.value === 'jsonpath'
})

const subjectPlaceholder = computed(() => {
  if (subjectType.value === 'header') return 'e.g. Content-Type'
  if (subjectType.value === 'jsonpath') return 'e.g. $.data.id'
  return ''
})

const needsExpected = computed(() => {
  return !['exists', 'not exists'].includes(operator.value)
})

const builtField = computed(() => {
  if (subjectType.value === 'header' && subjectExtra.value) {
    return `header.${subjectExtra.value}`
  }
  if (subjectType.value === 'jsonpath' && subjectExtra.value) {
    return subjectExtra.value
  }
  return subjectType.value
})

const builtLine = computed(() => {
  const parts = [builtField.value, operator.value]
  if (needsExpected.value && expected.value) {
    parts.push(expected.value)
  }
  return parts.join(' ')
})

function handleAdd() {
  if (!builtField.value) return

  emit('add', {
    subject: builtField.value,
    operator: operator.value,
    expected: needsExpected.value ? expected.value : '',
    line: 0,
  })

  // Reset form
  subjectExtra.value = ''
  expected.value = ''
}

function copyAsHitspec() {
  const lines = assertions.map((a) => {
    const parts = [a.subject, a.operator]
    if (a.expected) parts.push(String(a.expected))
    return parts.join(' ')
  })
  emit('copy', lines.join('\n'))
}

const groupedSubjects = computed(() => {
  const groups = new Map<string, typeof ASSERTION_SUBJECTS[number][]>()
  for (const s of ASSERTION_SUBJECTS) {
    if (!groups.has(s.group)) groups.set(s.group, [])
    groups.get(s.group)!.push(s)
  }
  return groups
})
</script>

<template>
  <div class="space-y-3">
    <div class="flex items-center justify-between">
      <h3 class="text-sm font-medium text-foreground">Assertions</h3>
      <div class="flex items-center gap-2">
        <button
          v-if="assertions.length > 0"
          class="flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground"
          @click="copyAsHitspec"
        >
          <Copy :size="12" /> Copy as .http
        </button>
        <button
          class="flex items-center gap-1 rounded-md bg-accent px-2 py-1 text-xs font-medium text-accent-foreground hover:bg-accent/80"
          @click="showBuilder = !showBuilder"
        >
          <Plus :size="12" /> Add
          <ChevronDown :size="12" :class="showBuilder ? 'rotate-180' : ''" class="transition-transform" />
        </button>
      </div>
    </div>

    <!-- Builder Form -->
    <div v-if="showBuilder" class="rounded-lg border border-border bg-background p-3 space-y-3">
      <div class="grid grid-cols-1 gap-3 sm:grid-cols-3">
        <!-- Subject -->
        <div>
          <label class="mb-1 block text-xs text-muted-foreground">Subject</label>
          <select
            v-model="subjectType"
            class="w-full rounded-md border border-border bg-input px-2 py-1.5 text-sm text-foreground focus:outline-none focus:ring-1 focus:ring-ring"
          >
            <template v-for="[group, subjects] in groupedSubjects" :key="group">
              <optgroup :label="group">
                <option v-for="s in subjects" :key="s.value" :value="s.value">{{ s.label }}</option>
              </optgroup>
            </template>
          </select>
        </div>

        <!-- Operator -->
        <div>
          <label class="mb-1 block text-xs text-muted-foreground">Operator</label>
          <select
            v-model="operator"
            class="w-full rounded-md border border-border bg-input px-2 py-1.5 text-sm text-foreground focus:outline-none focus:ring-1 focus:ring-ring"
          >
            <option v-for="op in ASSERTION_OPERATORS" :key="op.value" :value="op.value">
              {{ op.label }} ({{ op.value }})
            </option>
          </select>
        </div>

        <!-- Expected -->
        <div>
          <label class="mb-1 block text-xs text-muted-foreground">Expected</label>
          <input
            v-if="needsExpected"
            v-model="expected"
            type="text"
            placeholder="Expected value"
            class="w-full rounded-md border border-border bg-input px-2 py-1.5 text-sm text-foreground focus:outline-none focus:ring-1 focus:ring-ring"
          />
          <span v-else class="block py-1.5 text-xs italic text-muted-foreground">No value needed</span>
        </div>
      </div>

      <!-- Extra field (header name, jsonpath) -->
      <div v-if="needsExtra">
        <label class="mb-1 block text-xs text-muted-foreground">
          {{ subjectType === 'header' ? 'Header Name' : 'JSONPath Expression' }}
        </label>
        <input
          v-model="subjectExtra"
          type="text"
          :placeholder="subjectPlaceholder"
          class="w-full rounded-md border border-border bg-input px-2 py-1.5 text-sm text-foreground font-mono focus:outline-none focus:ring-1 focus:ring-ring"
        />
      </div>

      <!-- Preview -->
      <div class="flex items-center justify-between rounded border border-border bg-background px-3 py-2">
        <code class="text-xs text-foreground">{{ builtLine }}</code>
        <button
          class="rounded bg-accent px-3 py-1 text-xs font-medium text-accent-foreground hover:bg-accent/80 disabled:opacity-50"
          :disabled="!builtField"
          @click="handleAdd"
        >
          Add Assertion
        </button>
      </div>
    </div>

    <!-- Existing assertions list -->
    <div v-if="assertions.length > 0" class="space-y-1">
      <div
        v-for="(assertion, i) in assertions"
        :key="i"
        class="group flex items-center gap-2 rounded-md border border-border bg-background px-3 py-2 font-mono text-sm"
      >
        <span class="text-accent">{{ assertion.subject }}</span>
        <span class="text-nord-15">{{ assertion.operator }}</span>
        <span class="text-nord-14">{{ assertion.expected }}</span>
        <span class="ml-auto flex items-center gap-2">
          <span v-if="assertion.line" class="text-xs text-nord-3">L{{ assertion.line }}</span>
          <button
            class="invisible text-muted-foreground hover:text-destructive group-hover:visible"
            @click="emit('remove', i)"
          >
            <Trash2 :size="12" />
          </button>
        </span>
      </div>
    </div>
    <p v-else-if="!showBuilder" class="text-sm text-muted-foreground">
      No assertions defined. Click "Add" to build assertions visually.
    </p>
  </div>
</template>
