<script setup>
import { computed, ref } from 'vue'
import { Search } from 'lucide-vue-next'
import ConversationItem from './ConversationItem.vue'
import { useConversationsStore } from '@/stores/conversations'

const conversations = useConversationsStore()
const keyword = ref('')
const filtered = computed(() => conversations.items.filter((item) => `${item.title} ${item.id}`.toLowerCase().includes(keyword.value.toLowerCase())))
</script>

<template>
  <section class="w-[302px] border-r border-white/70 bg-[#f6fbff] p-4">
    <div class="mb-4 flex items-center justify-between">
      <div>
        <h1 class="text-lg font-semibold">Agent Chat</h1>
        <p class="text-xs text-slate-500">主会话 sess_ / 子会话 subsess_</p>
      </div>
      <button class="rounded-xl bg-blue-500 px-3 py-2 text-sm text-white shadow-sm" aria-label="新建会话">+</button>
    </div>
    <label class="mb-3 flex items-center gap-2 rounded-2xl bg-white px-3 py-2 text-sm text-slate-500 shadow-sm">
      <Search class="h-4 w-4" />
      <input v-model="keyword" class="w-full bg-transparent outline-none" placeholder="搜索会话或 session id" />
    </label>
    <div class="mb-3 flex gap-2 text-xs">
      <span class="rounded-full bg-blue-100 px-3 py-1 text-blue-700">全部</span>
      <span class="rounded-full bg-white px-3 py-1 text-slate-500">主 Agent</span>
      <span class="rounded-full bg-white px-3 py-1 text-slate-500">Subagent</span>
    </div>
    <div class="scrollbar-thin-blue flex h-[calc(100vh-150px)] flex-col gap-2 overflow-y-auto pr-1">
      <ConversationItem v-for="item in filtered" :key="item.id" :conversation="item" :active="item.id === conversations.activeSessionId" @select="conversations.setActiveSession(item.id)" />
    </div>
  </section>
</template>
