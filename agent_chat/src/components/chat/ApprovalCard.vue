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
  <article class="rounded-2xl border border-amber-200 bg-amber-50 p-4 text-sm text-amber-900">
    <div class="flex items-center gap-2 font-semibold"><ShieldCheck class="h-4 w-4" /> {{ event.title ?? '权限审批请求' }}</div>
    <p class="mt-2">{{ event.summary }}</p>
    <div v-if="event.decision === 'pending'" class="mt-3 flex flex-wrap gap-2">
      <button class="rounded-xl bg-amber-600 px-3 py-1.5 text-xs font-medium text-white" @click="decide('approved_once')">允许一次</button>
      <button class="rounded-xl bg-white px-3 py-1.5 text-xs font-medium text-amber-800 ring-1 ring-amber-200" @click="decide('always_approved')">总是允许</button>
      <button class="rounded-xl bg-white px-3 py-1.5 text-xs font-medium text-rose-700 ring-1 ring-rose-200" @click="decide('rejected')">拒绝</button>
    </div>
    <span v-else class="mt-3 inline-flex rounded-full bg-white px-2 py-1 text-xs">decision: {{ event.decision }}</span>
  </article>
</template>
