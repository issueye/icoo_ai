import { defineStore } from 'pinia'
import { agentBridge } from '@/services/agentBridge'

const defaultLevelFilter = 'all'
const defaultTypeFilter = 'all'
const defaultSessionFilter = 'all'

function normalizeAuditLevel(rawLevel) {
  const normalized = String(rawLevel || '').trim().toLowerCase()
  if (normalized === 'warning') return 'warn'
  if (normalized === 'debug' || normalized === 'info' || normalized === 'warn' || normalized === 'error') return normalized
  return 'info'
}

function normalizeAuditType(rawType) {
  const normalized = String(rawType || '').trim()
  return normalized || 'audit'
}

function normalizeTimestamp(rawValue) {
  const candidate = String(rawValue || '').trim()
  if (!candidate) return new Date().toISOString()
  const parsed = new Date(candidate)
  if (Number.isNaN(parsed.getTime())) return new Date().toISOString()
  return parsed.toISOString()
}

function normalizeAuditEvent(rawEvent = {}) {
  const createdAt = normalizeTimestamp(rawEvent?.createdAt)
  const type = normalizeAuditType(rawEvent?.type || rawEvent?.safeMeta?.gatewayType)
  const level = normalizeAuditLevel(rawEvent?.level || rawEvent?.status || rawEvent?.safeMeta?.level || rawEvent?.safeMeta?.severity)
  const summary = String(rawEvent?.summary || rawEvent?.content || '').trim() || '审计事件'
  const sessionId = String(rawEvent?.sessionId || '').trim()
  const id = String(rawEvent?.id || '').trim() || `audit_${sessionId || 'global'}_${type}_${createdAt}_${summary}`
  return {
    id,
    sessionId,
    type,
    level,
    summary,
    createdAt,
  }
}

function eventTimestamp(event) {
  const parsed = new Date(event?.createdAt || '').getTime()
  if (Number.isNaN(parsed)) return 0
  return parsed
}

function dedupeAndSortAuditEvents(events = []) {
  const byID = new Map()
  events.forEach((rawEvent) => {
    const event = normalizeAuditEvent(rawEvent)
    byID.set(event.id, event)
  })
  return Array.from(byID.values()).sort((a, b) => eventTimestamp(b) - eventTimestamp(a))
}

export const useAuditStore = defineStore('audit', {
  state: () => ({
    items: [],
    loading: false,
    error: null,
    lastLoadedAt: null,
    lastViewedAt: null,
    query: '',
    levelFilter: defaultLevelFilter,
    typeFilter: defaultTypeFilter,
    sessionFilter: defaultSessionFilter,
    selectedEventID: null,
  }),
  getters: {
    totalCount: (state) => state.items.length,
    unreadCount: (state) => {
      if (!state.items.length) return 0
      const viewedAt = new Date(state.lastViewedAt || '').getTime()
      if (Number.isNaN(viewedAt) || !state.lastViewedAt) return state.items.length
      return state.items.filter((item) => eventTimestamp(item) > viewedAt).length
    },
    levelOptions: (state) => {
      const levels = new Set(['debug', 'info', 'warn', 'error'])
      state.items.forEach((item) => {
        const level = normalizeAuditLevel(item.level)
        if (level) levels.add(level)
      })
      const ordered = ['all', ...Array.from(levels)]
      return ordered.map((item) => ({
        id: item,
        label: item === 'all' ? '全部等级' : item.toUpperCase(),
      }))
    },
    typeOptions: (state) => {
      const types = new Set()
      state.items.forEach((item) => {
        const type = normalizeAuditType(item.type)
        if (type) types.add(type)
      })
      return [
        { id: 'all', label: '全部类型' },
        ...Array.from(types).sort((a, b) => a.localeCompare(b)).map((item) => ({
          id: item,
          label: item,
        })),
      ]
    },
    sessionOptions: (state) => {
      const sessions = new Set()
      state.items.forEach((item) => {
        const sessionId = String(item.sessionId || '').trim()
        if (sessionId) sessions.add(sessionId)
      })
      return [
        { id: 'all', label: '全部会话' },
        ...Array.from(sessions).sort((a, b) => a.localeCompare(b)).map((item) => ({
          id: item,
          label: item,
        })),
      ]
    },
    filteredItems: (state) => {
      const keyword = String(state.query || '').trim().toLowerCase()
      return state.items.filter((item) => {
        const level = normalizeAuditLevel(item.level)
        const type = normalizeAuditType(item.type)
        const sessionId = String(item.sessionId || '').trim()
        if (state.levelFilter !== 'all' && level !== state.levelFilter) return false
        if (state.typeFilter !== 'all' && type !== state.typeFilter) return false
        if (state.sessionFilter !== 'all' && sessionId !== state.sessionFilter) return false
        if (!keyword) return true
        const haystack = `${item.id} ${sessionId} ${type} ${level} ${item.summary}`.toLowerCase()
        return haystack.includes(keyword)
      })
    },
    selectedEvent(state) {
      const target = this.filteredItems.find((item) => item.id === state.selectedEventID)
      if (target) return target
      return this.filteredItems[0] || null
    },
  },
  actions: {
    setQuery(value) {
      this.query = String(value || '')
    },
    setLevelFilter(value) {
      this.levelFilter = String(value || '').trim() || defaultLevelFilter
    },
    setTypeFilter(value) {
      this.typeFilter = String(value || '').trim() || defaultTypeFilter
    },
    setSessionFilter(value) {
      this.sessionFilter = String(value || '').trim() || defaultSessionFilter
    },
    setSelectedEvent(eventID) {
      this.selectedEventID = String(eventID || '').trim() || null
    },
    markViewed() {
      this.lastViewedAt = new Date().toISOString()
    },
    clearFilters() {
      this.query = ''
      this.levelFilter = defaultLevelFilter
      this.typeFilter = defaultTypeFilter
      this.sessionFilter = defaultSessionFilter
    },
    upsertAuditEvent(rawEvent) {
      const event = normalizeAuditEvent(rawEvent)
      const index = this.items.findIndex((item) => item.id === event.id)
      if (index >= 0) {
        this.items[index] = { ...this.items[index], ...event }
      } else {
        this.items.unshift(event)
      }
      this.items.sort((a, b) => eventTimestamp(b) - eventTimestamp(a))
      if (!this.selectedEventID) this.selectedEventID = event.id
      return event
    },
    applyGatewayEvent(event) {
      if (!event || typeof event !== 'object') return
      if (event.kind !== 'audit') return
      this.upsertAuditEvent(event)
    },
    async loadAuditEvents() {
      this.loading = true
      this.error = null
      try {
        const events = await agentBridge.listAuditEvents()
        this.items = dedupeAndSortAuditEvents(events)
        this.lastLoadedAt = new Date().toISOString()
        if (!this.selectedEventID || !this.items.some((item) => item.id === this.selectedEventID)) {
          this.selectedEventID = this.items[0]?.id || null
        }
      } catch (error) {
        this.error = error?.message || '加载审计日志失败'
      } finally {
        this.loading = false
      }
    },
  },
})
