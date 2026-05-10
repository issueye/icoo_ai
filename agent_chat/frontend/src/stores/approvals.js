import { defineStore } from 'pinia'
import { agentBridge } from '@/services/agentBridge'
import { useMessagesStore } from './messages'

function normalizeApprovalDecision(event) {
  const decision = String(event?.decision || '').trim()
  if (decision) return decision
  const status = String(event?.status || '').trim().toLowerCase()
  if (status === 'approved' || status === 'approved_once' || status === 'always_approved') return status
  if (status === 'rejected' || status === 'denied') return 'rejected'
  return 'pending'
}

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
      this.upsertApproval(result)
      const messages = useMessagesStore()
      messages.markApprovalDecision(payload.id, payload.decision)
      return result
    },
    upsertApproval(approval) {
      if (!approval || typeof approval !== 'object') return
      const approvalID = String(approval.id || '').trim()
      if (!approvalID) return
      const index = this.items.findIndex((item) => item.id === approvalID)
      if (index >= 0) {
        this.items[index] = { ...this.items[index], ...approval }
      } else {
        this.items.push(approval)
      }
    },
    applyGatewayEvent(event) {
      if (!event || typeof event !== 'object') return
      if (event.kind !== 'approval') return
      const approval = {
        id: String(event.id || '').trim(),
        sessionId: String(event.sessionId || '').trim(),
        decision: normalizeApprovalDecision(event),
        actor: 'gateway',
        summary: String(event.summary || event.content || '').trim(),
        createdAt: event.createdAt || new Date().toISOString(),
      }
      if (!approval.id) return
      this.upsertApproval(approval)
    },
  },
})
