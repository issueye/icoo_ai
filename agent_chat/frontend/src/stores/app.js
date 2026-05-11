import { defineStore } from 'pinia'
import { agentBridge } from '@/services/agentBridge'

const supportedChannelTypes = ['qq', 'lark', 'wechat']

const channelTemplates = {
  qq: { id: 'qq', name: 'QQ 机器人', type: 'qq' },
  lark: { id: 'lark', name: '飞书机器人', type: 'lark' },
  wechat: { id: 'wechat', name: '微信机器人', type: 'wechat' },
}

function normalizeChannelType(rawType, fallbackType = 'qq') {
  const source = typeof rawType === 'string' ? rawType.trim().toLowerCase() : ''
  if (supportedChannelTypes.includes(source)) return source
  const fallback = typeof fallbackType === 'string' ? fallbackType.trim().toLowerCase() : ''
  if (supportedChannelTypes.includes(fallback)) return fallback
  return 'qq'
}

function defaultChannelByType(channelType, sequence = 1) {
  const type = normalizeChannelType(channelType)
  const template = channelTemplates[type]
  const baseID = template?.id || type
  const id = sequence > 1 ? `${baseID}_${sequence}` : baseID
  return {
    id,
    name: template?.name || baseID,
    type,
    enabled: false,
    appId: '',
    appSecret: '',
    botToken: '',
    webhookUrl: '',
  }
}

function normalizeChannel(rawChannel = {}, fallbackType = 'qq') {
  const type = normalizeChannelType(rawChannel?.type, fallbackType)
  const defaults = defaultChannelByType(type, 1)
  const normalizedID = typeof rawChannel?.id === 'string' ? rawChannel.id.trim() : ''
  const normalizedName = typeof rawChannel?.name === 'string' ? rawChannel.name.trim() : ''
  const normalizedAppID = typeof rawChannel?.appId === 'string' ? rawChannel.appId.trim() : ''
  const normalizedAppSecret = typeof rawChannel?.appSecret === 'string' ? rawChannel.appSecret.trim() : ''
  const normalizedBotToken = typeof rawChannel?.botToken === 'string' ? rawChannel.botToken.trim() : ''
  const normalizedWebhookURL = typeof rawChannel?.webhookUrl === 'string' ? rawChannel.webhookUrl.trim() : ''
  return {
    id: normalizedID || defaults.id,
    name: normalizedName || defaults.name,
    type,
    enabled: Boolean(rawChannel?.enabled),
    appId: normalizedAppID,
    appSecret: normalizedAppSecret,
    botToken: normalizedBotToken,
    webhookUrl: normalizedWebhookURL,
  }
}

function ensureUniqueChannelIDs(channels = []) {
  const used = new Map()
  return channels.map((channel) => {
    const type = normalizeChannelType(channel?.type)
    const baseID = String(channel?.id || '').trim() || type
    if (!used.has(baseID)) {
      used.set(baseID, 1)
      return { ...channel, id: baseID, type }
    }
    const next = used.get(baseID) + 1
    used.set(baseID, next)
    const derivedID = `${baseID}_${next}`
    used.set(derivedID, 1)
    return { ...channel, id: derivedID, type }
  })
}

function normalizeChannels(rawChannels) {
  const source = Array.isArray(rawChannels) ? rawChannels : []
  if (source.length === 0) {
    return [
      defaultChannelByType('qq', 1),
      defaultChannelByType('lark', 1),
      defaultChannelByType('wechat', 1),
    ]
  }
  const normalized = source.map((rawChannel, index) => {
    const fallbackType = supportedChannelTypes[index % supportedChannelTypes.length] || 'qq'
    return normalizeChannel(rawChannel, fallbackType)
  })
  return ensureUniqueChannelIDs(normalized)
}

function normalizeMcpServer(rawServer = {}, sequence = 1) {
  const fallbackID = `mcp_${sequence}`
  const id = typeof rawServer?.id === 'string' ? rawServer.id.trim() : ''
  const name = typeof rawServer?.name === 'string' ? rawServer.name.trim() : ''
  const command = typeof rawServer?.command === 'string' ? rawServer.command.trim() : ''
  const args = Array.isArray(rawServer?.args)
    ? rawServer.args.map((item) => String(item ?? '').trim()).filter(Boolean)
    : []
  return {
    id: id || fallbackID,
    name: name || id || fallbackID,
    command,
    args,
    enabled: Boolean(rawServer?.enabled),
  }
}

function normalizeMcpServers(rawServers) {
  const source = Array.isArray(rawServers) ? rawServers : []
  if (source.length === 0) return []
  const used = new Map()
  return source.map((rawServer, index) => {
    const normalized = normalizeMcpServer(rawServer, index + 1)
    const baseID = normalized.id || `mcp_${index + 1}`
    if (!used.has(baseID)) {
      used.set(baseID, 1)
      return { ...normalized, id: baseID, name: normalized.name || baseID }
    }
    const next = used.get(baseID) + 1
    used.set(baseID, next)
    const derivedID = `${baseID}_${next}`
    used.set(derivedID, 1)
    return { ...normalized, id: derivedID, name: normalized.name || derivedID }
  })
}

function normalizeScheduleTask(rawTask = {}, sequence = 1) {
  const fallbackID = `task_${sequence}`
  const id = typeof rawTask?.id === 'string' ? rawTask.id.trim() : ''
  const name = typeof rawTask?.name === 'string' ? rawTask.name.trim() : ''
  const spec = typeof rawTask?.spec === 'string' ? rawTask.spec.trim() : ''
  const command = typeof rawTask?.command === 'string' ? rawTask.command.trim() : ''
  const args = Array.isArray(rawTask?.args)
    ? rawTask.args.map((item) => String(item ?? '').trim()).filter(Boolean)
    : []
  return {
    id: id || fallbackID,
    name: name || id || fallbackID,
    spec: spec || '*/5 * * * *',
    command,
    args,
    enabled: Boolean(rawTask?.enabled),
  }
}

function normalizeScheduleTasks(rawTasks) {
  const source = Array.isArray(rawTasks) ? rawTasks : []
  if (source.length === 0) return []
  const used = new Map()
  return source.map((rawTask, index) => {
    const normalized = normalizeScheduleTask(rawTask, index + 1)
    const baseID = normalized.id || `task_${index + 1}`
    if (!used.has(baseID)) {
      used.set(baseID, 1)
      return { ...normalized, id: baseID, name: normalized.name || baseID }
    }
    const next = used.get(baseID) + 1
    used.set(baseID, next)
    const derivedID = `${baseID}_${next}`
    used.set(derivedID, 1)
    return { ...normalized, id: derivedID, name: normalized.name || derivedID }
  })
}

export const useAppStore = defineStore('app', {
  state: () => ({
    activeNav: 'chats',
    bridgeStatus: 'connecting',
    sidebarFilter: 'all',
    gatewayStatus: 'gateway_connecting',
    gatewaySummary: '等待网关连接',
    gatewayUpdatedAt: null,
    gatewayBinaryPath: '',
    gatewayHost: '127.0.0.1',
    gatewayPort: 17889,
    acpEnabled: false,
    acpCommand: '',
    acpArgs: '',
    logLevel: 'info',
    logFormat: 'text',
    logFilePath: 'logs/agent_chat.log',
    channels: normalizeChannels([]),
    mcpServers: normalizeMcpServers([]),
    scheduleTasks: normalizeScheduleTasks([]),
    settingsSaving: false,
    settingsError: null,
    toasts: [],
  }),
  actions: {
    setActiveNav(value) { this.activeNav = value },
    setSidebarFilter(value) { this.sidebarFilter = value },
    applyGatewayStatusSnapshot(status) {
      this.gatewayStatus = status?.status || 'gateway_connecting'
      this.gatewaySummary = status?.summary || ''
      this.gatewayUpdatedAt = status?.updatedAt || null
      this.bridgeStatus = this.gatewayStatus === 'gateway_ready' ? 'gateway' : 'degraded'
    },
    applyGatewayEvent(event) {
      if (!event || typeof event !== 'object') return
      if (event.kind !== 'gateway_event') return
      this.applyGatewayStatusSnapshot({
        status: event.status || this.gatewayStatus,
        summary: event.summary || this.gatewaySummary,
        updatedAt: event.createdAt || new Date().toISOString(),
      })
    },
    async refreshGatewayStatus() {
      try {
        this.applyGatewayStatusSnapshot(await agentBridge.getGatewayStatus())
      } catch {
        this.gatewayStatus = 'gateway_failed'
        this.gatewaySummary = '无法获取网关状态'
        this.bridgeStatus = 'degraded'
      }
    },
    async restartGateway() {
      this.settingsError = null
      try {
        const status = await agentBridge.restartGateway()
        this.applyGatewayStatusSnapshot(status)
        return status
      } catch (error) {
        this.gatewayStatus = 'gateway_failed'
        this.gatewaySummary = error?.message || '网关重启失败'
        this.bridgeStatus = 'degraded'
        throw error
      }
    },
    async stopGateway() {
      this.settingsError = null
      try {
        const status = await agentBridge.stopGateway()
        this.applyGatewayStatusSnapshot({
          status: status?.status || 'gateway_failed',
          summary: status?.summary || '网关已关闭',
          updatedAt: status?.updatedAt || null,
        })
        return status
      } catch (error) {
        this.gatewayStatus = 'gateway_failed'
        this.gatewaySummary = error?.message || '网关关闭失败'
        this.bridgeStatus = 'degraded'
        throw error
      }
    },
    async loadAppSettings() {
      this.settingsError = null
      try {
        const settings = await agentBridge.getAppSettings()
        this.gatewayBinaryPath = settings?.gatewayBinaryPath || ''
        this.gatewayHost = settings?.gatewayHost || '127.0.0.1'
        const loadedPort = Number(settings?.gatewayPort)
        this.gatewayPort = Number.isFinite(loadedPort) && loadedPort > 0 ? loadedPort : 17889
        this.acpEnabled = Boolean(settings?.acpEnabled)
        this.acpCommand = settings?.acpCommand || ''
        this.acpArgs = settings?.acpArgs || ''
        this.logLevel = settings?.logLevel || 'info'
        this.logFormat = settings?.logFormat || 'text'
        this.logFilePath = settings?.logFilePath || 'logs/agent_chat.log'
        this.channels = normalizeChannels(settings?.channels)
        this.mcpServers = normalizeMcpServers(settings?.mcpServers)
        this.scheduleTasks = normalizeScheduleTasks(settings?.scheduleTasks)
      } catch (error) {
        this.settingsError = error?.message || '加载配置失败'
      }
    },
    async saveAppSettings(payload = {}) {
      this.settingsSaving = true
      this.settingsError = null
      try {
        const saved = await agentBridge.updateAppSettings({
          gatewayBinaryPath: payload.gatewayBinaryPath ?? this.gatewayBinaryPath ?? '',
          gatewayHost: payload.gatewayHost ?? this.gatewayHost ?? '127.0.0.1',
          gatewayPort: payload.gatewayPort ?? this.gatewayPort ?? 17889,
          acpEnabled: payload.acpEnabled ?? this.acpEnabled ?? false,
          acpCommand: payload.acpCommand ?? this.acpCommand ?? '',
          acpArgs: payload.acpArgs ?? this.acpArgs ?? '',
          logLevel: payload.logLevel ?? this.logLevel ?? 'info',
          logFormat: payload.logFormat ?? this.logFormat ?? 'text',
          logFilePath: payload.logFilePath ?? this.logFilePath ?? 'logs/agent_chat.log',
          channels: normalizeChannels(payload.channels ?? this.channels),
          mcpServers: normalizeMcpServers(payload.mcpServers ?? this.mcpServers),
          scheduleTasks: normalizeScheduleTasks(payload.scheduleTasks ?? this.scheduleTasks),
        })
        this.gatewayBinaryPath = saved?.gatewayBinaryPath || ''
        this.gatewayHost = saved?.gatewayHost || '127.0.0.1'
        const savedPort = Number(saved?.gatewayPort)
        this.gatewayPort = Number.isFinite(savedPort) && savedPort > 0 ? savedPort : 17889
        this.acpEnabled = Boolean(saved?.acpEnabled)
        this.acpCommand = saved?.acpCommand || ''
        this.acpArgs = saved?.acpArgs || ''
        this.logLevel = saved?.logLevel || 'info'
        this.logFormat = saved?.logFormat || 'text'
        this.logFilePath = saved?.logFilePath || 'logs/agent_chat.log'
        this.channels = normalizeChannels(saved?.channels)
        this.mcpServers = normalizeMcpServers(saved?.mcpServers)
        this.scheduleTasks = normalizeScheduleTasks(saved?.scheduleTasks)
        return saved
      } catch (error) {
        this.settingsError = error?.message || '保存配置失败'
        throw error
      } finally {
        this.settingsSaving = false
      }
    },
    pushToast(payload = {}) {
      const id = `toast_${Date.now()}_${Math.floor(Math.random() * 1000)}`
      const toast = {
        id,
        type: payload.type || 'info',
        message: payload.message || '',
      }
      this.toasts.push(toast)
      const duration = Number(payload.duration ?? 1800)
      setTimeout(() => {
        this.removeToast(id)
      }, Number.isFinite(duration) && duration > 0 ? duration : 1800)
      return id
    },
    removeToast(id) {
      this.toasts = this.toasts.filter((item) => item.id !== id)
    },
  },
})
