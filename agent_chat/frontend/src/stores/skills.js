import { defineStore } from 'pinia'
import { agentBridge } from '@/services/agentBridge'

export const useSkillsStore = defineStore('skills', {
  state: () => ({ items: [], loading: false }),
  actions: {
    async loadSkills() {
      this.loading = true
      try {
        this.items = await agentBridge.listSkills()
      } finally {
        this.loading = false
      }
    },
  },
})
