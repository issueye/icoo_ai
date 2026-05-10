import { defineStore } from 'pinia'
import { agentBridge } from '@/services/agentBridge'

const DEFAULT_MODE_OPTIONS = [
  { id: 'agent', label: 'Agent' },
]

const DEFAULT_MODEL_OPTIONS = [
  { id: 'gpt-5.4', label: 'GPT-5.4' },
]

function dedupeModels(agentProfiles = []) {
  const set = new Set()
  const out = []
  for (const profile of agentProfiles) {
    const models = Array.isArray(profile?.models) ? profile.models : []
    for (const rawModel of models) {
      const model = String(rawModel || '').trim()
      if (!model || set.has(model)) continue
      set.add(model)
      out.push({ id: model, label: model })
    }
  }
  return out
}

export const useConversationsStore = defineStore('conversations', {
  state: () => ({
    items: [],
    activeSessionId: null,
    filter: 'all',
    loading: false,
    error: null,
    agentProfiles: [],
    workspaceOptions: [
      { id: 'workspace_current', label: '当前仓库', path: 'E:/codes/icoo_ai' },
      { id: 'workspace_agent_chat', label: 'agent_chat', path: 'E:/codes/icoo_ai/agent_chat' },
      { id: 'workspace_agent_server', label: 'agent_server', path: 'E:/codes/icoo_ai/agent_server' },
    ],
    modeOptions: [...DEFAULT_MODE_OPTIONS],
    modelOptions: [...DEFAULT_MODEL_OPTIONS],
  }),
  getters: {
    activeConversation: (state) => state.items.find((item) => item.id === state.activeSessionId) ?? state.items[0],
    activeWorkspace: (state) => {
      const conversation = state.items.find((item) => item.id === state.activeSessionId) ?? state.items[0]
      return state.workspaceOptions.find((item) => item.id === conversation?.workspaceId) ?? state.workspaceOptions[0]
    },
    activeMode: (state) => {
      const conversation = state.items.find((item) => item.id === state.activeSessionId) ?? state.items[0]
      return state.modeOptions.find((item) => item.id === conversation?.mode) ?? state.modeOptions[0] ?? DEFAULT_MODE_OPTIONS[0]
    },
    activeModel: (state) => {
      const conversation = state.items.find((item) => item.id === state.activeSessionId) ?? state.items[0]
      if (conversation?.model) {
        const matched = state.modelOptions.find((item) => item.id === conversation.model)
        if (matched) return matched
      }
      return state.modelOptions[0] ?? DEFAULT_MODEL_OPTIONS[0]
    },
    mainConversations: (state) => state.items.filter((item) => item.id.startsWith('sess_')),
    subagentConversations: (state) => state.items.filter((item) => item.id.startsWith('subsess_')),
    filteredItems: (state) => {
      if (state.filter === 'main') return state.items.filter((item) => item.id.startsWith('sess_'))
      if (state.filter === 'subagent') return state.items.filter((item) => item.id.startsWith('subsess_'))
      if (state.filter === 'failed') return state.items.filter((item) => item.status === 'failed')
      return state.items
    },
  },
  actions: {
    normalizeMode(mode) {
      const normalized = String(mode || '').trim()
      if (!normalized) return this.modeOptions[0]?.id ?? DEFAULT_MODE_OPTIONS[0].id
      const matched = this.modeOptions.find((item) => item.id === normalized)
      return matched?.id ?? normalized
    },
    normalizeModel(model) {
      const normalized = String(model || '').trim()
      if (!normalized) return this.modelOptions[0]?.id ?? DEFAULT_MODEL_OPTIONS[0].id
      const matched = this.modelOptions.find((item) => item.id === normalized)
      return matched?.id ?? normalized
    },
    applyAgentProfiles(profiles = []) {
      const normalizedProfiles = Array.isArray(profiles)
        ? profiles
          .map((item) => {
            const id = String(item?.id || '').trim()
            if (!id) return null
            return {
              id,
              name: String(item?.name || '').trim() || id,
              protocol: String(item?.protocol || '').trim(),
              description: String(item?.description || '').trim(),
              models: Array.isArray(item?.models)
                ? item.models.map((model) => String(model || '').trim()).filter(Boolean)
                : [],
            }
          })
          .filter(Boolean)
        : []

      this.agentProfiles = normalizedProfiles

      const dynamicModeOptions = normalizedProfiles.map((profile) => ({
        id: profile.id,
        label: profile.name,
      }))
      this.modeOptions = dynamicModeOptions.length ? dynamicModeOptions : [...DEFAULT_MODE_OPTIONS]

      const dynamicModelOptions = dedupeModels(normalizedProfiles)
      this.modelOptions = dynamicModelOptions.length ? dynamicModelOptions : [...DEFAULT_MODEL_OPTIONS]
    },
    async loadAgentProfiles() {
      try {
        const profiles = await agentBridge.listAgents()
        this.applyAgentProfiles(profiles)
      } catch (error) {
        this.applyAgentProfiles([])
        this.error = error?.message ?? '加载 Agent 列表失败'
      }
    },
    async loadConversations() {
      this.loading = true
      this.error = null
      try {
        this.items = await agentBridge.listConversations()
        if (!this.items.some((item) => item.id === this.activeSessionId)) {
          this.activeSessionId = this.items[0]?.id ?? null
        }
      } catch (error) {
        this.error = error?.message ?? '加载会话失败'
      } finally {
        this.loading = false
      }
    },
    async createConversation(payload = {}) {
      const normalizedCwd = typeof payload.cwd === 'string' ? payload.cwd.trim() : ''
      const workspaceId = payload.workspaceId ?? (normalizedCwd ? this.ensureWorkspaceOption(normalizedCwd) : undefined)
      const mode = this.normalizeMode(payload.mode ?? this.activeMode?.id)
      const model = this.normalizeModel(payload.model ?? this.activeModel?.id)
      const conversation = await agentBridge.newSession({
        ...payload,
        cwd: normalizedCwd || payload.cwd,
        workspaceId,
        mode,
        model,
      })
      this.upsertConversation(conversation, true)
      this.activeSessionId = conversation.id
      return conversation
    },
    setActiveSession(sessionId) {
      if (sessionId) this.activeSessionId = sessionId
    },
    setFilter(filter) {
      this.filter = filter
    },
    updateActiveContext(patch) {
      if (!this.activeSessionId) return
      const index = this.items.findIndex((item) => item.id === this.activeSessionId)
      if (index < 0) return
      const workspace = patch.workspaceId ? this.workspaceOptions.find((item) => item.id === patch.workspaceId) : null
      const mode = Object.prototype.hasOwnProperty.call(patch, 'mode') ? this.normalizeMode(patch.mode) : this.items[index].mode
      const model = Object.prototype.hasOwnProperty.call(patch, 'model') ? this.normalizeModel(patch.model) : this.items[index].model
      this.items[index] = {
        ...this.items[index],
        ...patch,
        mode,
        model,
        cwd: workspace?.path ?? this.items[index].cwd,
        updatedAt: new Date().toISOString(),
      }
    },
    upsertConversation(conversation, prepend = false) {
      const index = this.items.findIndex((item) => item.id === conversation.id)
      if (index >= 0) {
        this.items[index] = { ...this.items[index], ...conversation }
        return
      }
      if (prepend) this.items.unshift(conversation)
      else this.items.push(conversation)
    },
    ensureWorkspaceOption(path) {
      const normalized = String(path || '').trim()
      if (!normalized) return this.activeWorkspace?.id ?? this.workspaceOptions[0]?.id
      const existing = this.workspaceOptions.find((item) => item.path.toLowerCase() === normalized.toLowerCase())
      if (existing) return existing.id
      const fallbackLabel = normalized.replace(/\\/g, '/').split('/').filter(Boolean).pop() || '自定义工作区'
      const id = `workspace_custom_${Date.now()}`
      this.workspaceOptions.push({ id, label: fallbackLabel, path: normalized })
      return id
    },
  },
})
