<script setup lang="ts">
import type { MockRouteDTO } from '@/types/api'
import MethodBadge from '@/components/common/MethodBadge.vue'

defineProps<{ routes: MockRouteDTO[] }>()
</script>

<template>
  <div class="rounded-lg border border-border bg-background p-4">
    <h3 class="mb-3 text-sm font-medium text-foreground">Routes</h3>
    <div v-if="routes.length === 0" class="text-sm text-muted-foreground">
      No routes configured
    </div>
    <div v-else class="space-y-1">
      <div
        v-for="(route, i) in routes"
        :key="i"
        class="flex items-center gap-3 rounded-md border border-border bg-surface px-3 py-2"
      >
        <MethodBadge :method="route.method" />
        <span class="font-mono text-sm text-foreground">{{ route.path }}</span>
        <span class="ml-auto text-xs text-muted-foreground">
          {{ route.statusCode }} | {{ route.contentType }}
        </span>
      </div>
    </div>
  </div>
</template>
