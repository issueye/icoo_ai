<script setup>
import { ShieldCheck } from 'lucide-vue-next'
import { useApprovalsStore } from '@/stores/approvals'

const props = defineProps({ event: { type: Object, required: true } })
const approvals = useApprovalsStore()

function decide(decision) {
  approvals.decide({ id: props.event.id, sessionId: props.event.sessionId, decision })
}
</script>

<template>
  <article class="qq-event-card is-warning">
    <div class="flex items-center gap-2 font-semibold"><ShieldCheck class="h-4 w-4" /> {{ event.title ?? '权限审批请求' }}</div>
    <p class="mt-2">{{ event.summary }}</p>
    <div v-if="event.decision === 'pending'" class="mt-3 flex flex-wrap gap-2">
      <button class="qq-primary-action px-3 py-1.5 text-xs" @click="decide('approved_once')">允许一次</button>
      <button class="qq-icon-button px-3 text-xs font-medium" @click="decide('always_approved')">总是允许</button>
      <button class="qq-icon-button px-3 text-xs font-medium text-[color:var(--qq-danger)]" @click="decide('rejected')">拒绝</button>
    </div>
    <span v-else class="qq-session-pill mt-3 inline-flex">decision: {{ event.decision }}</span>
  </article>
</template>
