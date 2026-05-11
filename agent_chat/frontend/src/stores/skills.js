import { defineStore } from 'pinia'
import { agentBridge } from '@/services/agentBridge'

export const useSkillsStore = defineStore('skills', {
  state: () => ({
    items: [],
    loading: false,
    error: '',
    query: '',
    lastLoadedAt: '',
  }),
  getters: {
    filteredItems(state) {
      const keyword = String(state.query || '').trim().toLowerCase()
      if (!keyword) return state.items
      return state.items.filter((item) => {
        const id = String(item?.id || '').toLowerCase()
        const name = String(item?.name || '').toLowerCase()
        const description = String(item?.description || '').toLowerCase()
        return id.includes(keyword) || name.includes(keyword) || description.includes(keyword)
      })
    },
  },
  actions: {
    setQuery(value) {
      this.query = String(value || '')
    },
    async loadSkills() {
      this.loading = true
      this.error = ''
      try {
        const result = await agentBridge.listSkills()
        this.items = Array.isArray(result) ? result : []
        this.lastLoadedAt = new Date().toISOString()
      } catch (error) {
        this.error = error instanceof Error ? error.message : '技能列表加载失败'
        this.items = []
      } finally {
        this.loading = false
      }
    },
  },
})
