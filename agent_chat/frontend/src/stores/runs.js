import { defineStore } from 'pinia'
import { agentBridge } from '@/services/agentBridge'

function normalizeRunStatus(status) {
  const normalized = String(status || '').trim().toLowerCase()
  if (!normalized) return 'running'
  return normalized
}

function runStatusLabel(status) {
  switch (normalizeRunStatus(status)) {
    case 'running':
      return '运行中'
    case 'completed':
      return '已完成'
    case 'failed':
      return '已失败'
    case 'cancelled':
      return '已取消'
    case 'waiting_approval':
      return '等待审批'
    case 'queued':
      return '排队中'
    default:
      return status || '未知状态'
  }
}

function isTerminalStatus(status) {
  const normalized = normalizeRunStatus(status)
  return normalized === 'completed' || normalized === 'failed' || normalized === 'cancelled'
}

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
      this.upsertRun(run)
      return run
    },
    upsertRun(run) {
      if (!run || typeof run !== 'object') return
      const runID = String(run.id || '').trim()
      if (!runID) return
      const index = this.items.findIndex((item) => item.id === runID)
      if (index >= 0) {
        this.items[index] = { ...this.items[index], ...run }
      } else {
        this.items.push(run)
      }
    },
    applyGatewayEvent(event) {
      if (!event || typeof event !== 'object') return
      if (event.kind !== 'run') return
      const status = String(event.status || '').trim() || 'running'
      const createdAt = event.createdAt || new Date().toISOString()
      const run = {
        id: String(event.id || '').trim(),
        sessionId: String(event.sessionId || '').trim(),
        status,
        label: runStatusLabel(status),
        startedAt: createdAt,
        completedAt: isTerminalStatus(status) ? createdAt : null,
      }
      if (!run.id) return
      const index = this.items.findIndex((item) => item.id === run.id)
      if (index >= 0) {
        const previous = this.items[index]
        this.items[index] = {
          ...previous,
          ...run,
          startedAt: previous.startedAt || run.startedAt,
          completedAt: run.completedAt || previous.completedAt || null,
        }
        return
      }
      this.items.push(run)
    },
  },
})
