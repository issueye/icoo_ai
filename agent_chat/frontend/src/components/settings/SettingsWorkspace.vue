<script setup>
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { Power, PowerOff, RefreshCcw, RotateCcw, Save } from 'lucide-vue-next'
import ContextDropdown from '@/components/ui/ContextDropdown.vue'
import SettingsMcpPanel from '@/components/settings/SettingsMcpPanel.vue'
import SettingsSchedulePanel from '@/components/settings/SettingsSchedulePanel.vue'
import { useAppStore } from '@/stores/app'

const app = useAppStore()
defineProps({
  section: {
    type: String,
    default: 'gateway',
  },
})
const gatewayPath = ref('')
const gatewayHost = ref('127.0.0.1')
const gatewayPort = ref(17889)
const acpEnabled = ref('disabled')
const acpCommand = ref('')
const acpArgs = ref('')
const logLevel = ref('info')
const logFormat = ref('text')
const logFilePath = ref('logs/agent_chat.log')
const confirmDialogOpen = ref(false)
const confirmDialogTitle = ref('')
const confirmDialogLines = ref([])
const confirmDialogConfirmLabel = ref('确认')
const confirmDialogCancelLabel = ref('取消')

const logLevelOptions = [
  { id: 'debug', label: 'debug' },
  { id: 'info', label: 'info' },
  { id: 'warn', label: 'warn' },
  { id: 'error', label: 'error' },
]

const logFormatOptions = [
  { id: 'text', label: 'text' },
  { id: 'json', label: 'json' },
]

const acpModeOptions = [
  { id: 'enabled', label: '启用 ACP' },
  { id: 'disabled', label: '禁用 ACP' },
]

let confirmDialogResolver = null

onMounted(async () => {
  await app.loadAppSettings()
  gatewayPath.value = app.gatewayBinaryPath || ''
  gatewayHost.value = app.gatewayHost || '127.0.0.1'
  gatewayPort.value = Number(app.gatewayPort || 17889)
  acpEnabled.value = app.acpEnabled ? 'enabled' : 'disabled'
  acpCommand.value = app.acpCommand || ''
  acpArgs.value = app.acpArgs || ''
  logLevel.value = app.logLevel || 'info'
  logFormat.value = app.logFormat || 'text'
  logFilePath.value = app.logFilePath || 'logs/agent_chat.log'
})

const disabled = computed(() => app.settingsSaving)
const statusLabel = computed(() => {
  const mapping = {
    gateway_connecting: '连接中',
    gateway_ready: '已连接',
    gateway_reconnecting: '重连中',
    gateway_failed: '连接失败',
  }
  return mapping[app.gatewayStatus] ?? '未知'
})

const statusClass = computed(() => {
  const mapping = {
    gateway_connecting: 'is-connecting',
    gateway_ready: 'is-ready',
    gateway_reconnecting: 'is-reconnecting',
    gateway_failed: 'is-failed',
  }
  return mapping[app.gatewayStatus] ?? 'is-connecting'
})

function requestGatewayConfirmation({ title, lines, confirmLabel, cancelLabel }) {
  confirmDialogTitle.value = title
  confirmDialogLines.value = Array.isArray(lines) ? lines : []
  confirmDialogConfirmLabel.value = confirmLabel || '确认'
  confirmDialogCancelLabel.value = cancelLabel || '取消'
  confirmDialogOpen.value = true
  return new Promise((resolve) => {
    confirmDialogResolver = resolve
  })
}

function resolveGatewayConfirmation(confirmed) {
  confirmDialogOpen.value = false
  if (typeof confirmDialogResolver === 'function') {
    const resolver = confirmDialogResolver
    confirmDialogResolver = null
    resolver(Boolean(confirmed))
  }
}

onBeforeUnmount(() => {
  if (typeof confirmDialogResolver === 'function') {
    confirmDialogResolver(false)
    confirmDialogResolver = null
  }
})

async function saveSettings() {
  try {
    const normalizedPort = Number(gatewayPort.value)
    const normalizedHost = gatewayHost.value.trim() || '127.0.0.1'
    const normalizedBinaryPath = gatewayPath.value.trim()
    const normalizedGatewayPort = Number.isFinite(normalizedPort) ? normalizedPort : 17889
    const normalizedACPEnabled = acpEnabled.value === 'enabled'
    const normalizedACPCommand = acpCommand.value.trim()
    const normalizedACPArgs = acpArgs.value.trim()
    if (normalizedACPEnabled && !normalizedACPCommand) {
      app.pushToast({ type: 'error', message: '启用 ACP 时必须填写 ACP 命令' })
      return
    }
    const normalizedLogLevel = ['debug', 'info', 'warn', 'error'].includes((logLevel.value || '').trim().toLowerCase())
      ? (logLevel.value || '').trim().toLowerCase()
      : 'info'
    const normalizedLogFormat = ['text', 'json'].includes((logFormat.value || '').trim().toLowerCase())
      ? (logFormat.value || '').trim().toLowerCase()
      : 'text'
    const normalizedLogFilePath = logFilePath.value.trim() || 'logs/agent_chat.log'
    const settingsChanged =
      normalizedBinaryPath !== (app.gatewayBinaryPath || '') ||
      normalizedHost !== (app.gatewayHost || '127.0.0.1') ||
      normalizedGatewayPort !== Number(app.gatewayPort || 17889) ||
      normalizedACPEnabled !== Boolean(app.acpEnabled) ||
      normalizedACPCommand !== (app.acpCommand || '') ||
      normalizedACPArgs !== (app.acpArgs || '') ||
      normalizedLogLevel !== (app.logLevel || 'info') ||
      normalizedLogFormat !== (app.logFormat || 'text') ||
      normalizedLogFilePath !== (app.logFilePath || 'logs/agent_chat.log')

    await app.saveAppSettings({
      gatewayBinaryPath: normalizedBinaryPath,
      gatewayHost: normalizedHost,
      gatewayPort: normalizedGatewayPort,
      acpEnabled: normalizedACPEnabled,
      acpCommand: normalizedACPCommand,
      acpArgs: normalizedACPArgs,
      logLevel: normalizedLogLevel,
      logFormat: normalizedLogFormat,
      logFilePath: normalizedLogFilePath,
    })

    if (settingsChanged) {
      const shouldRestart = await requestGatewayConfirmation({
        title: '配置已保存',
        lines: ['检测到网关或日志配置变更。', '是否立即重启网关以应用新配置？'],
        confirmLabel: '立即重启',
        cancelLabel: '稍后重启',
      })
      if (shouldRestart) {
        await app.restartGateway()
        app.pushToast({ type: 'success', message: '配置保存并已重启网关' })
      } else {
        await app.refreshGatewayStatus()
        app.pushToast({ type: 'info', message: '配置已保存，未重启网关' })
      }
      return
    }

    await app.refreshGatewayStatus()
    app.pushToast({ type: 'success', message: '配置保存成功（无变更）' })
  } catch {
    app.pushToast({ type: 'error', message: app.settingsError || '配置保存失败' })
  }
}

async function refreshGatewayStatus() {
  await app.refreshGatewayStatus()
  const isReady = app.gatewayStatus === 'gateway_ready'
  app.pushToast({
    type: isReady ? 'success' : 'info',
    message: isReady ? '网关刷新成功，连接正常' : `网关状态已刷新：${statusLabel.value}`,
  })
}

async function restartGateway() {
  try {
    const confirmed = await requestGatewayConfirmation({
      title: '重启网关',
      lines: ['将停止当前网关并重新拉起。', '是否继续重启？'],
      confirmLabel: '确认重启',
      cancelLabel: '取消',
    })
    if (!confirmed) return
    await app.restartGateway()
    app.pushToast({ type: 'success', message: '网关重启完成' })
  } catch {
    app.pushToast({ type: 'error', message: app.gatewaySummary || '网关重启失败' })
  }
}

async function stopGateway() {
  try {
    const confirmed = await requestGatewayConfirmation({
      title: '关闭网关',
      lines: ['关闭后将中断当前网关连接。', '是否确认关闭网关服务？'],
      confirmLabel: '确认关闭',
      cancelLabel: '取消',
    })
    if (!confirmed) return
    await app.stopGateway()
    app.pushToast({ type: 'success', message: '网关已关闭' })
  } catch {
    app.pushToast({ type: 'error', message: app.gatewaySummary || '网关关闭失败' })
  }
}

function resetToDefault() {
  gatewayPath.value = ''
  gatewayHost.value = '127.0.0.1'
  gatewayPort.value = 17889
  acpEnabled.value = 'disabled'
  acpCommand.value = ''
  acpArgs.value = ''
  logLevel.value = 'info'
  logFormat.value = 'text'
  logFilePath.value = 'logs/agent_chat.log'
  app.pushToast({ type: 'info', message: '已恢复默认配置，请保存后重启网关' })
}
</script>

<template>
  <section v-if="section === 'gateway'" class="qq-chat-workspace">
    <header class="qq-chat-header qq-settings-header">
      <div class="min-w-0 flex-1">
        <h2 class="qq-chat-title">配置</h2>
        <p class="qq-sidebar-subtitle">网关服务与日志参数</p>
      </div>
    </header>

    <div class="qq-settings-body">
      <div class="qq-settings-card">
        <label class="qq-settings-label" for="gatewayBinaryPath">网关可执行文件路径</label>
        <input
          id="gatewayBinaryPath"
          v-model="gatewayPath"
          type="text"
          class="qq-settings-input"
          placeholder="例如：E:/codes/icoo_ai/agent_gateway/runtime/gateway/agent-gateway.exe"
        />
        <label class="qq-settings-label" for="gatewayHost">网关 Host</label>
        <input
          id="gatewayHost"
          v-model="gatewayHost"
          type="text"
          class="qq-settings-input"
          placeholder="127.0.0.1"
        />
        <label class="qq-settings-label" for="gatewayPort">网关 Port</label>
        <input
          id="gatewayPort"
          v-model.number="gatewayPort"
          type="number"
          min="1"
          max="65535"
          class="qq-settings-input"
          placeholder="17889"
        />
        <label class="qq-settings-label" for="acpEnabled">ACP 模式</label>
        <ContextDropdown
          id="acpEnabled"
          v-model="acpEnabled"
          class="qq-settings-dropdown"
          label="ACP"
          :options="acpModeOptions"
        />
        <label class="qq-settings-label" for="acpCommand">ACP 命令</label>
        <input
          id="acpCommand"
          v-model="acpCommand"
          type="text"
          class="qq-settings-input"
          placeholder="例如：icoo-ai"
        />
        <label class="qq-settings-label" for="acpArgs">ACP 参数</label>
        <input
          id="acpArgs"
          v-model="acpArgs"
          type="text"
          class="qq-settings-input"
          placeholder="例如：serve --transport stdio"
        />
        <label class="qq-settings-label" for="logLevel">日志级别</label>
        <ContextDropdown
          id="logLevel"
          v-model="logLevel"
          class="qq-settings-dropdown"
          label="级别"
          :options="logLevelOptions"
        />
        <label class="qq-settings-label" for="logFormat">日志格式</label>
        <ContextDropdown
          id="logFormat"
          v-model="logFormat"
          class="qq-settings-dropdown"
          label="格式"
          :options="logFormatOptions"
        />
        <label class="qq-settings-label" for="logFilePath">日志文件路径</label>
        <input
          id="logFilePath"
          v-model="logFilePath"
          type="text"
          class="qq-settings-input"
          placeholder="logs/agent_chat.log"
        />
        <div class="qq-settings-actions">
          <button class="qq-icon-button" :disabled="disabled" aria-label="恢复默认路径" @click="resetToDefault">
            <RotateCcw class="h-4 w-4" />
          </button>
          <button class="qq-icon-button" :disabled="disabled" aria-label="重启网关服务" @click="restartGateway">
            <Power class="h-4 w-4" />
          </button>
          <button class="qq-icon-button" :disabled="disabled" aria-label="关闭网关服务" @click="stopGateway">
            <PowerOff class="h-4 w-4" />
          </button>
          <button class="qq-icon-button" :disabled="disabled" aria-label="刷新网关状态" @click="refreshGatewayStatus">
            <RefreshCcw class="h-4 w-4" />
          </button>
          <button class="qq-primary-action h-9 px-4" :disabled="disabled" @click="saveSettings">
            <Save class="h-4 w-4" />
            <span>{{ app.settingsSaving ? '保存中' : '保存配置' }}</span>
          </button>
          <span v-if="app.settingsError" class="qq-settings-error">{{ app.settingsError }}</span>
        </div>
      </div>
    </div>

    <div v-if="confirmDialogOpen" class="qq-modal-backdrop" @click.self="resolveGatewayConfirmation(false)">
      <div class="qq-modal" role="dialog" aria-modal="true" aria-labelledby="gatewayConfirmTitle">
        <div class="qq-modal-header">
          <h3 id="gatewayConfirmTitle" class="qq-modal-title">{{ confirmDialogTitle }}</h3>
          <button class="qq-icon-button" type="button" aria-label="关闭弹窗" @click="resolveGatewayConfirmation(false)">×</button>
        </div>
        <div class="qq-modal-body">
          <p v-for="(line, index) in confirmDialogLines" :key="`${line}_${index}`" class="qq-modal-summary">{{ line }}</p>
        </div>
        <div class="qq-modal-actions">
          <button class="qq-secondary-action h-8 px-3 text-sm" type="button" @click="resolveGatewayConfirmation(false)">{{ confirmDialogCancelLabel }}</button>
          <button class="qq-primary-action h-8 px-3 text-sm" type="button" @click="resolveGatewayConfirmation(true)">{{ confirmDialogConfirmLabel }}</button>
        </div>
      </div>
    </div>
  </section>
  <SettingsMcpPanel v-else-if="section === 'mcp'" />
  <SettingsSchedulePanel v-else-if="section === 'schedule'" />
</template>
