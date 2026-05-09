import { defineStore } from 'pinia'
import { mockConversations } from '@/services/mockData'

export const useConversationsStore = defineStore('conversations', {
  state: () => ({ items: mockConversations, activeSessionId: 'sess_main_20260509_001' }),
  getters: {
    activeConversation: (state) => state.items.find((item) => item.id === state.activeSessionId) ?? state.items[0],
    mainConversations: (state) => state.items.filter((item) => item.id.startsWith('sess_')),
    subagentConversations: (state) => state.items.filter((item) => item.id.startsWith('subsess_')),
  },
  actions: {
    setActiveSession(sessionId) { this.activeSessionId = sessionId },
  },
})
