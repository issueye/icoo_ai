<script setup>
import { computed } from 'vue'
import { AlertTriangle, Bell, Info } from 'lucide-vue-next'

const props = defineProps({
  event: { type: Object, required: true },
})

const tone = computed(() => {
  if (props.event?.kind === 'gateway_event') {
    return props.event?.status === 'gateway_failed' ? 'danger' : 'info'
  }
  const level = String(props.event?.level || '').toLowerCase()
  if (level === 'error' || level === 'critical' || level === 'warn' || level === 'warning') return 'danger'
  if (level === 'notice') return 'notice'
  return 'info'
})

const icon = computed(() => {
  if (tone.value === 'danger') return AlertTriangle
  if (tone.value === 'notice') return Bell
  return Info
})

const title = computed(() => {
  if (props.event?.kind === 'gateway_event') return '网关状态提醒'
  if (props.event?.kind === 'audit') return '审计提醒'
  return '系统提醒'
})

const summary = computed(() => props.event?.summary || props.event?.content || '收到一条系统事件')
const meta = computed(() => props.event?.type || props.event?.status || '')
</script>

<template>
  <article class="qq-ued-reminder" :class="`is-${tone}`">
    <component :is="icon" class="qq-ued-reminder-icon" />
    <div class="qq-ued-reminder-body">
      <header class="qq-ued-reminder-title-row">
        <h4 class="qq-ued-reminder-title">{{ title }}</h4>
        <span v-if="meta" class="qq-ued-reminder-tag">{{ meta }}</span>
      </header>
      <p class="qq-ued-reminder-summary">{{ summary }}</p>
    </div>
  </article>
</template>
