<script setup>
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'
import { Search } from 'lucide-vue-next'
import ConversationItem from './ConversationItem.vue'
import ContextDropdown from '@/components/ui/ContextDropdown.vue'
import { useConversationsStore } from '@/stores/conversations'
import { useMessagesStore } from '@/stores/messages'
import { useAppStore } from '@/stores/app'

const conversations = useConversationsStore()
const messages = useMessagesStore()
const app = useAppStore()
const router = useRouter()
const keyword = ref('')
const filters = [
  { key: 'all', label: '全部' },
  { key: 'main', label: '主 Agent' },
  { key: 'subagent', label: 'Subagent' },
]
const filtered = computed(() => conversations.filteredItems.filter((item) => `${item.title} ${item.id}`.toLowerCase().includes(keyword.value.toLowerCase())))
const creating = ref(false)
const dialogOpen = ref(false)
const formError = ref('')
const sessionTitle = ref('新的 Agent 会话')
const workspaceDir = ref('')
const selectedAgentId = ref('')
const agentOptions = computed(() => conversations.agentProfiles.map((agent) => ({
  id: agent.id,
  label: `${agent.name} (${agent.id})`,
})))

function openCreateDialog() {
  sessionTitle.value = '新的 Agent 会话'
  workspaceDir.value = conversations.activeWorkspace?.path ?? ''
  selectedAgentId.value = conversations.activeMode?.id || conversations.agentProfiles[0]?.id || ''
  formError.value = ''
  dialogOpen.value = true
}

function closeCreateDialog() {
  if (creating.value) return
  dialogOpen.value = false
}

async function createConversation() {
  const cwd = workspaceDir.value.trim()
  if (!cwd) {
    formError.value = '请输入工作区目录'
    return
  }
  if (!selectedAgentId.value.trim()) {
    formError.value = '请选择 Agent'
    return
  }
  creating.value = true
  formError.value = ''
  try {
    const conversation = await conversations.createConversation({
      title: sessionTitle.value.trim() || '新的 Agent 会话',
      cwd,
      agentId: selectedAgentId.value.trim(),
      mode: selectedAgentId.value.trim(),
    })
    router.push(`/chats/${conversation.id}`)
  } catch (error) {
    formError.value = error?.message ?? '创建会话失败'
  } finally {
    creating.value = false
  }
}

function selectConversation(sessionId) {
  conversations.setActiveSession(sessionId)
  router.push(`/chats/${sessionId}`)
}

async function connectConversation(item) {
  try {
    const conversation = await conversations.connectConversation(item.id, {
      cwd: item.cwd || conversations.activeWorkspace?.path || '',
    })
    await messages.loadMessages(conversation.id)
    router.push(`/chats/${conversation.id}`)
    app.pushToast({ type: 'success', message: '已通过 ACP 连接会话' })
  } catch (error) {
    app.pushToast({ type: 'error', message: error?.message || '连接会话失败' })
  }
}

async function disconnectConversation(item) {
  const confirmed = window.confirm(`确认断开会话「${item.title || item.id}」吗？`)
  if (!confirmed) return
  try {
    await conversations.disconnectConversation(item.id)
    app.pushToast({ type: 'success', message: '会话已断开' })
  } catch (error) {
    app.pushToast({ type: 'error', message: error?.message || '断开会话失败' })
  }
}

async function deleteConversation(item) {
  const confirmed = window.confirm(`确认删除会话「${item.title || item.id}」吗？此操作不可恢复。`)
  if (!confirmed) return
  try {
    await conversations.deleteConversation(item.id)
    messages.clearSession(item.id)
    if (conversations.activeSessionId) {
      router.push(`/chats/${conversations.activeSessionId}`)
    } else {
      router.push('/chats')
    }
    app.pushToast({ type: 'success', message: '会话已删除' })
  } catch (error) {
    app.pushToast({ type: 'error', message: error?.message || '删除会话失败' })
  }
}
</script>

<template>
  <section class="qq-sidebar">
    <div class="mb-4 flex items-center justify-between">
      <div>
        <h1 class="qq-sidebar-title">Agent Chat</h1>
        <p class="qq-sidebar-subtitle">主会话 sess_ / 子会话 subsess_</p>
      </div>
      <button class="qq-primary-action h-8 w-8 text-base" aria-label="新建会话" @click="openCreateDialog">+</button>
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
      <ConversationItem
        v-for="item in filtered"
        :key="item.id"
        :conversation="item"
        :active="item.id === conversations.activeSessionId"
        @select="selectConversation(item.id)"
        @connect="connectConversation(item)"
        @disconnect="disconnectConversation(item)"
        @delete="deleteConversation(item)"
      />
      <div v-if="!filtered.length" class="qq-event-card text-center text-sm">没有找到匹配会话</div>
    </div>

    <div v-if="dialogOpen" class="qq-modal-backdrop">
      <div class="qq-modal qq-create-session-modal">
        <div class="qq-modal-header">
          <h3 class="qq-modal-title">新建会话</h3>
          <button class="qq-icon-button" type="button" aria-label="关闭弹窗" @click="closeCreateDialog">×</button>
        </div>
        <div class="qq-modal-body">
          <label class="qq-modal-field">
            <span class="qq-modal-label">会话名称</span>
            <input v-model="sessionTitle" class="qq-settings-input" type="text" placeholder="新的 Agent 会话" />
          </label>
          <label class="qq-modal-field">
            <span class="qq-modal-label">工作区目录</span>
            <input v-model="workspaceDir" class="qq-settings-input" type="text" placeholder="E:/codes/icoo_ai" />
          </label>
          <label class="qq-modal-field">
            <span class="qq-modal-label">Agent</span>
            <ContextDropdown
              v-model="selectedAgentId"
              class="qq-settings-dropdown"
              label="Agent"
              :options="agentOptions"
            />
          </label>
          <p v-if="formError" class="qq-settings-error">{{ formError }}</p>
        </div>
        <div class="qq-modal-actions">
          <button class="qq-secondary-action h-8 px-3 text-sm" type="button" :disabled="creating" @click="closeCreateDialog">取消</button>
          <button class="qq-primary-action h-8 px-3 text-sm" type="button" :disabled="creating" @click="createConversation">{{ creating ? '创建中' : '创建并连接' }}</button>
        </div>
      </div>
    </div>
  </section>
</template>
