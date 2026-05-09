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
  <section class="qq-sidebar">
    <div class="mb-4 flex items-center justify-between">
      <div>
        <h1 class="qq-sidebar-title">Agent Chat</h1>
        <p class="qq-sidebar-subtitle">主会话 sess_ / 子会话 subsess_</p>
      </div>
      <button class="qq-primary-action h-8 w-8 text-base" aria-label="新建会话" @click="createConversation">+</button>
    </div>
    <label class="qq-search-box mb-3 text-sm">
      <Search class="h-4 w-4" />
      <input v-model="keyword" class="w-full bg-transparent outline-none" placeholder="搜索会话或 session id" />
    </label>
    <div class="mb-3 flex gap-2 text-xs">
      <button v-for="filter in filters" :key="filter.key" class="qq-chip" :class="{ 'is-active': conversations.filter === filter.key }" @click="conversations.setFilter(filter.key)">
        {{ filter.label }}
      </button>
    </div>
    <div class="scrollbar-thin-blue flex h-[calc(100vh-var(--global-header-height)-150px)] flex-col gap-2 overflow-y-auto pr-1">
      <ConversationItem v-for="item in filtered" :key="item.id" :conversation="item" :active="item.id === conversations.activeSessionId" @select="selectConversation(item.id)" />
      <div v-if="!filtered.length" class="qq-event-card text-center text-sm">没有找到匹配会话</div>
    </div>
  </section>
</template>
