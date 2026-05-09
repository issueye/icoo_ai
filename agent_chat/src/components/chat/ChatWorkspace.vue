<script setup>
import ChatHeader from './ChatHeader.vue'
import ComposerPanel from './ComposerPanel.vue'
import MessageTimeline from './MessageTimeline.vue'
</script>

<template>
  <section class="qq-chat-workspace">
    <ChatHeader />
    <MessageTimeline />
    <ComposerPanel />
  </section>
</template>
