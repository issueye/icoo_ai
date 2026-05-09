<script setup>
import { computed } from 'vue'

const props = defineProps({
  message: { type: Object, required: true },
})

const isUser = computed(() => props.message?.role === 'user')
const roleLabel = computed(() => (isUser.value ? '我' : 'AI'))

const timeText = computed(() => {
  const raw = props.message?.createdAt
  if (!raw) return '--:--'
  const date = raw instanceof Date ? raw : new Date(raw)
  if (Number.isNaN(date.getTime())) return '--:--'
  return `${String(date.getHours()).padStart(2, '0')}:${String(date.getMinutes()).padStart(2, '0')}`
})
</script>

<template>
  <article class="qq-ued-message-row" :class="isUser ? 'is-user' : 'is-assistant'">
    <div class="qq-ued-message-avatar" :class="isUser ? 'is-user' : 'is-assistant'">{{ roleLabel }}</div>
    <section class="qq-ued-message-body">
      <header class="qq-ued-message-meta">
        <span class="qq-ued-message-role">{{ isUser ? '用户' : 'Agent' }}</span>
        <time class="qq-ued-message-time">{{ timeText }}</time>
      </header>
      <p class="qq-ued-message-content">{{ message.content }}</p>
    </section>
  </article>
</template>
