import { defineStore } from 'pinia'
import { agentBridge } from '@/services/agentBridge'
import { useMessagesStore } from './messages'

export const useApprovalsStore = defineStore('approvals', {
  state: () => ({ items: [], loading: false }),
  getters: {
    pendingCount: (state) => state.items.filter((item) => item.decision === 'pending').length,
  },
  actions: {
    async loadApprovals() {
      this.loading = true
      try {
        this.items = await agentBridge.listApprovals()
      } finally {
        this.loading = false
      }
    },
    async decide(payload) {
      const result = await agentBridge.decideApproval(payload)
      const index = this.items.findIndex((item) => item.id === payload.id)
      if (index >= 0) this.items[index] = { ...this.items[index], ...result }
      const messages = useMessagesStore()
      messages.markApprovalDecision(payload.id, payload.decision)
      return result
    },
  },
})
