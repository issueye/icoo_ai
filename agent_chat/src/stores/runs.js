import { defineStore } from 'pinia'
import { agentBridge } from '@/services/agentBridge'

export const useRunsStore = defineStore('runs', {
  state: () => ({ items: [], loading: false }),
  getters: {
    bySessionId: (state) => (sessionId) => state.items.filter((item) => item.sessionId === sessionId),
  },
  actions: {
    async loadRuns() {
      this.loading = true
      try {
        this.items = await agentBridge.listRuns()
      } finally {
        this.loading = false
      }
    },
    async cancel(sessionId) {
      const run = await agentBridge.cancel(sessionId)
      this.items.push(run)
      return run
    },
  },
})
