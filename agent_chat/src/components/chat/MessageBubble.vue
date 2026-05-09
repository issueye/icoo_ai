<script setup>
defineProps({ message: { type: Object, required: true } })
</script>

<template>
  <div class="flex" :class="message.role === 'user' ? 'justify-end' : 'justify-start'">
    <div class="max-w-[72%] rounded-3xl px-4 py-3 text-sm leading-6 shadow-sm" :class="message.role === 'user' ? 'rounded-br-md bg-blue-500 text-white' : 'rounded-bl-md bg-white text-slate-700'">
      {{ message.content }}
    </div>
  </div>
</template>
