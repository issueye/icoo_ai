<script setup>
defineProps({ message: { type: Object, required: true } })
</script>

<template>
  <div class="flex" :class="message.role === 'user' ? 'justify-end' : 'justify-start'">
    <div class="qq-message-bubble" :class="message.role === 'user' ? 'is-user' : 'is-agent'">
      {{ message.content }}
    </div>
  </div>
</template>
