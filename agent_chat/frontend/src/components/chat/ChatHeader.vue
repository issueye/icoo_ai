<script setup>
import { Boxes, BrainCircuit, FolderGit2, GitBranch, MoreHorizontal, Search } from 'lucide-vue-next'
import ContextDropdown from '@/components/ui/ContextDropdown.vue'
import { useConversationsStore } from '@/stores/conversations'

const conversations = useConversationsStore()
</script>

<template>
  <header class="qq-chat-header">
    <div class="min-w-0 flex-1">
      <div class="flex items-center gap-2">
        <h2 class="qq-chat-title">{{ conversations.activeConversation?.title }}</h2>
        <span class="qq-status-pill">{{ conversations.activeConversation?.status }}</span>
      </div>
      <p class="mt-1 flex items-center gap-2 text-xs text-[color:var(--qq-text-muted)]">
        <GitBranch class="h-3.5 w-3.5" />
        {{ conversations.activeSessionId }}
        <span v-if="conversations.activeConversation?.parentSessionId">父会话 {{ conversations.activeConversation.parentSessionId }}</span>
      </p>
    </div>
    <div class="flex gap-2">
      <button class="qq-icon-button" aria-label="搜索消息"><Search class="h-5 w-5" /></button>
      <button class="qq-icon-button" aria-label="更多操作"><MoreHorizontal class="h-5 w-5" /></button>
    </div>
  </header>
</template>
