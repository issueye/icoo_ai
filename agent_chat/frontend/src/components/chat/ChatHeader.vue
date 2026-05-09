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
      <div class="qq-context-bar">
        <ContextDropdown
          label="工作区"
          :icon="FolderGit2"
          :model-value="conversations.activeConversation?.workspaceId ?? conversations.activeWorkspace.id"
          :options="conversations.workspaceOptions"
          @update:model-value="conversations.updateActiveContext({ workspaceId: $event })"
        />
        <ContextDropdown
          label="模式"
          :icon="Boxes"
          :model-value="conversations.activeConversation?.mode ?? conversations.activeMode.id"
          :options="conversations.modeOptions"
          @update:model-value="conversations.updateActiveContext({ mode: $event })"
        />
        <ContextDropdown
          label="模型"
          :icon="BrainCircuit"
          :model-value="conversations.activeConversation?.model ?? conversations.activeModel.id"
          :options="conversations.modelOptions"
          @update:model-value="conversations.updateActiveContext({ model: $event })"
        />
      </div>
    </div>
    <div class="flex gap-2">
      <button class="qq-icon-button" aria-label="搜索消息"><Search class="h-5 w-5" /></button>
      <button class="qq-icon-button" aria-label="更多操作"><MoreHorizontal class="h-5 w-5" /></button>
    </div>
  </header>
</template>
