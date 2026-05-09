import { defineStore } from 'pinia'
import { agentBridge } from '@/services/agentBridge'
import { useConversationsStore } from './conversations'

export const useMessagesStore = defineStore('messages', {
  state: () => ({ items: [], loadingBySessionId: {}, sendingBySessionId: {}, error: null }),
  getters: {
    activeItems: (state) => {
      const conversations = useConversationsStore()
      return state.items.filter((item) => item.sessionId === conversations.activeSessionId)
    },
  },
  actions: {
    async loadMessages(sessionId) {
      if (!sessionId) return
      this.loadingBySessionId = { ...this.loadingBySessionId, [sessionId]: true }
      this.error = null
      try {
        const messages = await agentBridge.listMessages(sessionId)
        this.items = [...this.items.filter((item) => item.sessionId !== sessionId), ...messages]
      } catch (error) {
        this.error = error?.message ?? '加载消息失败'
      } finally {
        this.loadingBySessionId = { ...this.loadingBySessionId, [sessionId]: false }
      }
    },
    appendItems(items) {
      for (const item of items) {
        const index = this.items.findIndex((existing) => existing.id === item.id)
        if (index >= 0) this.items[index] = { ...this.items[index], ...item }
        else this.items.push(item)
      }
    },
    async sendPrompt(sessionId, content, context = {}) {
      const normalizedContent = content.trim()
      if (!sessionId || !normalizedContent) return []
      this.sendingBySessionId = { ...this.sendingBySessionId, [sessionId]: true }
      this.error = null
      try {
        const events = await agentBridge.prompt({ sessionId, content: normalizedContent, ...context })
        this.appendItems(events)
        return events
      } catch (error) {
        this.error = error?.message ?? '发送失败'
        return []
      } finally {
        this.sendingBySessionId = { ...this.sendingBySessionId, [sessionId]: false }
      }
    },
    markApprovalDecision(id, decision) {
      const event = this.items.find((item) => item.id === id)
      if (event) Object.assign(event, { decision, status: 'decided' })
    },
    clearSession(sessionId) {
      this.items = this.items.filter((item) => item.sessionId !== sessionId)
    },
  },
})
