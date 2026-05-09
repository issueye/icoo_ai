import { defineStore } from 'pinia'
import { agentBridge } from '@/services/agentBridge'

export const useConversationsStore = defineStore('conversations', {
  state: () => ({ items: [], activeSessionId: 'sess_main_20260509_001', filter: 'all', loading: false, error: null }),
  getters: {
    activeConversation: (state) => state.items.find((item) => item.id === state.activeSessionId) ?? state.items[0],
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
