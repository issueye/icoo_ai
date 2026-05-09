<script setup>
import AppNavRail from './AppNavRail.vue'
import ConversationSidebar from '@/components/conversation/ConversationSidebar.vue'
import ChatWorkspace from '@/components/chat/ChatWorkspace.vue'
</script>

<template>
  <main class="flex h-screen overflow-hidden bg-[#eaf3fb] text-slate-900">
    <AppNavRail />
    <ConversationSidebar />
    <ChatWorkspace />
  </main>
</template>
