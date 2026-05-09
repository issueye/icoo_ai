<script setup>
import ChatHeader from './ChatHeader.vue'
import ComposerPanel from './ComposerPanel.vue'
import MessageTimeline from './MessageTimeline.vue'
</script>

<template>
  <section class="flex min-w-0 flex-1 flex-col bg-[#eef6fd]">
    <ChatHeader />
    <MessageTimeline />
    <ComposerPanel />
  </section>
</template>
