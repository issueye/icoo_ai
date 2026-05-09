import { defineStore } from 'pinia'
import { agentBridge } from '@/services/agentBridge'

export const useAuditStore = defineStore('audit', {
  state: () => ({ items: [], loading: false }),
  actions: {
    async loadAuditEvents() {
      this.loading = true
      try {
        this.items = await agentBridge.listAuditEvents()
      } finally {
        this.loading = false
      }
    },
  },
})
