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
    async loadAppSettings() {
      this.settingsError = null
      try {
        const settings = await agentBridge.getAppSettings()
        this.gatewayBinaryPath = settings?.gatewayBinaryPath || ''
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
        })
        this.gatewayBinaryPath = saved?.gatewayBinaryPath || ''
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
