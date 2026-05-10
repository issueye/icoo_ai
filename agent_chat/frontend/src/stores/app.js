import { defineStore } from 'pinia'
import { agentBridge } from '@/services/agentBridge'

const channelTypeOrder = ['qq', 'lark', 'wechat']

const channelTemplates = {
  qq: { id: 'qq', name: 'QQ 机器人', type: 'qq' },
  lark: { id: 'lark', name: '飞书机器人', type: 'lark' },
  wechat: { id: 'wechat', name: '微信机器人', type: 'wechat' },
}

function normalizeChannelType(rawType, fallbackType = 'qq') {
  const source = typeof rawType === 'string' ? rawType.trim().toLowerCase() : ''
  if (channelTypeOrder.includes(source)) return source
  const fallback = typeof fallbackType === 'string' ? fallbackType.trim().toLowerCase() : ''
  if (channelTypeOrder.includes(fallback)) return fallback
  return 'qq'
}

function defaultChannelByType(channelType) {
  const type = normalizeChannelType(channelType)
  const template = channelTemplates[type]
  return {
    id: template.id,
    name: template.name,
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
  const defaults = defaultChannelByType(type)
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

function normalizeChannels(rawChannels) {
  const source = Array.isArray(rawChannels) ? rawChannels : []
  const channelsByType = new Map()
  source.forEach((rawChannel, index) => {
    const fallbackType = channelTypeOrder[index] || 'qq'
    const normalized = normalizeChannel(rawChannel, fallbackType)
    if (!channelsByType.has(normalized.type)) {
      channelsByType.set(normalized.type, normalized)
    }
  })
  return channelTypeOrder.map((type) => channelsByType.get(type) || defaultChannelByType(type))
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
