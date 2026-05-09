import { defineStore } from 'pinia'
import { agentBridge } from '@/services/agentBridge'

export const useConversationsStore = defineStore('conversations', {
  state: () => ({
    items: [],
    activeSessionId: 'sess_main_20260509_001',
    filter: 'all',
    loading: false,
    error: null,
    workspaceOptions: [
      { id: 'workspace_current', label: '当前仓库', path: 'E:/code/issueye/icoo_ai' },
      { id: 'workspace_agent_chat', label: 'agent_chat', path: 'E:/code/issueye/icoo_ai/agent_chat' },
      { id: 'workspace_agent_server', label: 'agent_server', path: 'E:/code/issueye/icoo_ai/agent_server' },
    ],
    modeOptions: [
      { id: 'chat', label: '聊天' },
      { id: 'agent', label: 'Agent' },
      { id: 'review', label: '审查' },
      { id: 'plan', label: '计划' },
    ],
    modelOptions: [
      { id: 'gpt-5.4', label: 'GPT-5.4' },
      { id: 'gpt-5.4-mini', label: 'GPT-5.4 Mini' },
      { id: 'gpt-5.3-codex', label: 'GPT-5.3 Codex' },
    ],
  }),
  getters: {
    activeConversation: (state) => state.items.find((item) => item.id === state.activeSessionId) ?? state.items[0],
    activeWorkspace: (state) => {
      const conversation = state.items.find((item) => item.id === state.activeSessionId) ?? state.items[0]
      return state.workspaceOptions.find((item) => item.id === conversation?.workspaceId) ?? state.workspaceOptions[0]
    },
    activeMode: (state) => {
      const conversation = state.items.find((item) => item.id === state.activeSessionId) ?? state.items[0]
      return state.modeOptions.find((item) => item.id === conversation?.mode) ?? state.modeOptions[1]
    },
    activeModel: (state) => {
      const conversation = state.items.find((item) => item.id === state.activeSessionId) ?? state.items[0]
      return state.modelOptions.find((item) => item.id === conversation?.model) ?? state.modelOptions[0]
    },
    mainConversations: (state) => state.items.filter((item) => item.id.startsWith('sess_')),
    subagentConversations: (state) => state.items.filter((item) => item.id.startsWith('subsess_')),
    filteredItems: (state) => {
      if (state.filter === 'main') return state.items.filter((item) => item.id.startsWith('sess_'))
      if (state.filter === 'subagent') return state.items.filter((item) => item.id.startsWith('subsess_'))
      if (state.filter === 'failed') return state.items.filter((item) => item.status === 'failed')
      return state.items
    },
  },
  actions: {
    async loadConversations() {
      this.loading = true
      this.error = null
      try {
        this.items = await agentBridge.listConversations()
        if (!this.items.some((item) => item.id === this.activeSessionId)) {
          this.activeSessionId = this.items[0]?.id ?? null
        }
      } catch (error) {
        this.error = error?.message ?? '加载会话失败'
      } finally {
        this.loading = false
      }
    },
    async createConversation(payload = {}) {
      const conversation = await agentBridge.newSession(payload)
      this.upsertConversation(conversation, true)
      this.activeSessionId = conversation.id
      return conversation
    },
    setActiveSession(sessionId) {
      if (sessionId) this.activeSessionId = sessionId
    },
    setFilter(filter) {
      this.filter = filter
    },
    updateActiveContext(patch) {
      if (!this.activeSessionId) return
      const index = this.items.findIndex((item) => item.id === this.activeSessionId)
      if (index < 0) return
      const workspace = patch.workspaceId ? this.workspaceOptions.find((item) => item.id === patch.workspaceId) : null
      this.items[index] = {
        ...this.items[index],
        ...patch,
        cwd: workspace?.path ?? this.items[index].cwd,
        updatedAt: new Date().toISOString(),
      }
    },
    upsertConversation(conversation, prepend = false) {
      const index = this.items.findIndex((item) => item.id === conversation.id)
      if (index >= 0) {
        this.items[index] = { ...this.items[index], ...conversation }
        return
      }
      if (prepend) this.items.unshift(conversation)
      else this.items.push(conversation)
    },
  },
})
