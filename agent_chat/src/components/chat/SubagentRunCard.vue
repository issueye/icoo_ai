<script setup>
import { GitBranch } from 'lucide-vue-next'
defineProps({ event: { type: Object, required: true } })
</script>

<template>
  <article class="rounded-2xl border border-violet-200 bg-violet-50 p-4 text-sm text-violet-950">
    <div class="flex items-center justify-between gap-3">
      <div class="flex items-center gap-2 font-semibold"><GitBranch class="h-4 w-4" /> Subagent Run</div>
      <span class="rounded-full bg-white px-2 py-0.5 text-xs">{{ event.status }}</span>
    </div>
    <p class="mt-2">{{ event.task }}</p>
    <p class="mt-1 text-xs text-violet-700">{{ event.subSessionId }} · 父会话 {{ event.parentSessionId }} · {{ event.eventCount }} events</p>
    <p class="mt-2 text-slate-700">{{ event.summary }}</p>
  </article>
</template>
