import { defineStore } from 'pinia'
import { mockMessages } from '@/services/mockData'
import { useConversationsStore } from './conversations'

export const useMessagesStore = defineStore('messages', {
  state: () => ({ items: mockMessages }),
  getters: {
    activeItems: (state) => {
      const conversations = useConversationsStore()
      return state.items.filter((item) => item.sessionId === conversations.activeSessionId)
    },
  },
})
