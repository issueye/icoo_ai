<script setup>
import { ShieldCheck } from 'lucide-vue-next'
defineProps({ event: { type: Object, required: true } })
</script>

<template>
  <article class="rounded-2xl border border-amber-200 bg-amber-50 p-4 text-sm text-amber-900">
    <div class="flex items-center gap-2 font-semibold"><ShieldCheck class="h-4 w-4" /> {{ event.title }}</div>
    <p class="mt-2">{{ event.summary }}</p>
    <span class="mt-3 inline-flex rounded-full bg-white px-2 py-1 text-xs">decision: {{ event.decision }}</span>
  </article>
</template>
