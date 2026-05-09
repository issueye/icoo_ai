import { defineStore } from 'pinia'
import { agentBridge } from '@/services/agentBridge'

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
    settingsSaving: false,
    settingsError: null,
    toasts: [],
  }),
  actions: {
    setActiveNav(value) { this.activeNav = value },
    setSidebarFilter(value) { this.sidebarFilter = value },
    async refreshGatewayStatus() {
      try {
        const status = await agentBridge.getGatewayStatus()
        this.gatewayStatus = status?.status || 'gateway_connecting'
        this.gatewaySummary = status?.summary || ''
        this.gatewayUpdatedAt = status?.updatedAt || null
        this.bridgeStatus = this.gatewayStatus === 'gateway_ready' ? 'gateway' : 'degraded'
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
        this.gatewayStatus = status?.status || 'gateway_connecting'
        this.gatewaySummary = status?.summary || ''
        this.gatewayUpdatedAt = status?.updatedAt || null
        this.bridgeStatus = this.gatewayStatus === 'gateway_ready' ? 'gateway' : 'degraded'
        return status
      } catch (error) {
        this.gatewayStatus = 'gateway_failed'
        this.gatewaySummary = error?.message || '网关重启失败'
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
        })
        this.gatewayBinaryPath = saved?.gatewayBinaryPath || ''
        this.gatewayHost = saved?.gatewayHost || '127.0.0.1'
        const savedPort = Number(saved?.gatewayPort)
        this.gatewayPort = Number.isFinite(savedPort) && savedPort > 0 ? savedPort : 17889
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
