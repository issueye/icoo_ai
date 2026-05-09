import { defineStore } from 'pinia'

export const useAppStore = defineStore('app', {
  state: () => ({ activeNav: 'chats', bridgeStatus: 'mock', sidebarFilter: 'all' }),
  actions: {
    setActiveNav(value) { this.activeNav = value },
    setSidebarFilter(value) { this.sidebarFilter = value },
  },
})
