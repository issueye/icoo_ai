<script setup>
import { GitBranch } from 'lucide-vue-next'
defineProps({ event: { type: Object, required: true } })
defineEmits(['open-session'])
</script>

<template>
  <article class="qq-event-card is-subagent">
    <div class="flex items-center justify-between gap-3">
      <div class="flex items-center gap-2 font-semibold"><GitBranch class="h-4 w-4" /> Subagent Run</div>
      <span class="qq-session-pill is-subagent">{{ event.status }}</span>
    </div>
    <p class="mt-2">{{ event.task }}</p>
    <p class="mt-1 text-xs text-[color:var(--qq-text-muted)]">{{ event.subSessionId }} · 父会话 {{ event.parentSessionId }} · {{ event.eventCount }} events</p>
    <p class="mt-2 text-[color:var(--qq-text-soft)]">{{ event.summary }}</p>
    <button class="qq-icon-button mt-3 px-3 text-xs font-medium text-[color:var(--qq-subagent)]" @click="$emit('open-session', event.subSessionId)">打开子会话</button>
  </article>
</template>
