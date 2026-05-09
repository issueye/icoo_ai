<script setup>
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'
import { Search } from 'lucide-vue-next'
import ConversationItem from './ConversationItem.vue'
import { useConversationsStore } from '@/stores/conversations'

const conversations = useConversationsStore()
const router = useRouter()
const keyword = ref('')
const filters = [
  { key: 'all', label: '全部' },
  { key: 'main', label: '主 Agent' },
  { key: 'subagent', label: 'Subagent' },
]
const filtered = computed(() => conversations.filteredItems.filter((item) => `${item.title} ${item.id}`.toLowerCase().includes(keyword.value.toLowerCase())))

async function createConversation() {
  const conversation = await conversations.createConversation({ title: '新的 Agent 会话' })
  router.push(`/chats/${conversation.id}`)
}

function selectConversation(sessionId) {
  conversations.setActiveSession(sessionId)
  router.push(`/chats/${sessionId}`)
}
</script>

<template>
  <section class="w-[302px] border-r border-white/70 bg-[#f6fbff] p-4">
    <div class="mb-4 flex items-center justify-between">
      <div>
        <h1 class="text-lg font-semibold">Agent Chat</h1>
        <p class="text-xs text-slate-500">主会话 sess_ / 子会话 subsess_</p>
      </div>
      <button class="rounded-xl bg-blue-500 px-3 py-2 text-sm text-white shadow-sm" aria-label="新建会话" @click="createConversation">+</button>
    </div>
    <label class="mb-3 flex items-center gap-2 rounded-2xl bg-white px-3 py-2 text-sm text-slate-500 shadow-sm">
      <Search class="h-4 w-4" />
      <input v-model="keyword" class="w-full bg-transparent outline-none" placeholder="搜索会话或 session id" />
    </label>
    <div class="mb-3 flex gap-2 text-xs">
      <button v-for="filter in filters" :key="filter.key" class="rounded-full px-3 py-1 transition" :class="conversations.filter === filter.key ? 'bg-blue-100 text-blue-700' : 'bg-white text-slate-500 hover:bg-blue-50'" @click="conversations.setFilter(filter.key)">
        {{ filter.label }}
      </button>
    </div>
    <div class="scrollbar-thin-blue flex h-[calc(100vh-150px)] flex-col gap-2 overflow-y-auto pr-1">
      <ConversationItem v-for="item in filtered" :key="item.id" :conversation="item" :active="item.id === conversations.activeSessionId" @select="selectConversation(item.id)" />
      <div v-if="!filtered.length" class="rounded-2xl bg-white px-4 py-6 text-center text-sm text-slate-500">没有找到匹配会话</div>
    </div>
  </section>
</template>
