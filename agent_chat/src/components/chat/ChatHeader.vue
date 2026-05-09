<script setup>
import { GitBranch, MoreHorizontal, Search } from 'lucide-vue-next'
import { useConversationsStore } from '@/stores/conversations'

const conversations = useConversationsStore()
</script>

<template>
  <header class="flex h-[72px] items-center justify-between border-b border-white/70 bg-[#f8fbff]/90 px-6">
    <div>
      <div class="flex items-center gap-2">
        <h2 class="text-lg font-semibold">{{ conversations.activeConversation?.title }}</h2>
        <span class="rounded-full bg-emerald-100 px-2 py-0.5 text-xs text-emerald-700">{{ conversations.activeConversation?.status }}</span>
      </div>
      <p class="mt-1 flex items-center gap-2 text-xs text-slate-500">
        <GitBranch class="h-3.5 w-3.5" />
        {{ conversations.activeSessionId }}
        <span v-if="conversations.activeConversation?.parentSessionId">父会话 {{ conversations.activeConversation.parentSessionId }}</span>
      </p>
    </div>
    <div class="flex gap-2">
      <button class="rounded-xl bg-white p-2 text-slate-500 shadow-sm" aria-label="搜索消息"><Search class="h-5 w-5" /></button>
      <button class="rounded-xl bg-white p-2 text-slate-500 shadow-sm" aria-label="更多操作"><MoreHorizontal class="h-5 w-5" /></button>
    </div>
  </header>
</template>
