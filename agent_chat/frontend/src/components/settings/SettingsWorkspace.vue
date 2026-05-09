<script setup>
import { computed, onMounted, ref } from 'vue'
import { Power, RefreshCcw, RotateCcw, Save } from 'lucide-vue-next'
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

onMounted(async () => {
  await app.loadAppSettings()
  gatewayPath.value = app.gatewayBinaryPath || ''
  gatewayHost.value = app.gatewayHost || '127.0.0.1'
  gatewayPort.value = Number(app.gatewayPort || 17889)
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

async function saveSettings() {
  try {
    const normalizedPort = Number(gatewayPort.value)
    const normalizedHost = gatewayHost.value.trim() || '127.0.0.1'
    const normalizedBinaryPath = gatewayPath.value.trim()
    const normalizedGatewayPort = Number.isFinite(normalizedPort) ? normalizedPort : 17889
    const settingsChanged =
      normalizedBinaryPath !== (app.gatewayBinaryPath || '') ||
      normalizedHost !== (app.gatewayHost || '127.0.0.1') ||
      normalizedGatewayPort !== Number(app.gatewayPort || 17889)

    await app.saveAppSettings({
      gatewayBinaryPath: normalizedBinaryPath,
      gatewayHost: normalizedHost,
      gatewayPort: normalizedGatewayPort,
    })

    if (settingsChanged) {
      const shouldRestart = globalThis?.confirm?.('配置已保存。是否立即重启网关以应用新配置？')
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
    await app.restartGateway()
    app.pushToast({ type: 'success', message: '网关重启完成' })
  } catch {
    app.pushToast({ type: 'error', message: app.gatewaySummary || '网关重启失败' })
  }
}

function resetToDefault() {
  gatewayPath.value = './runtime/gateway/agent-gateway.exe'
  gatewayHost.value = '127.0.0.1'
  gatewayPort.value = 17889
  app.pushToast({ type: 'info', message: '已恢复默认路径，请保存配置' })
}
</script>

<template>
  <section class="qq-chat-workspace">
    <header class="qq-chat-header qq-settings-header">
      <div class="min-w-0 flex-1">
        <h2 class="qq-chat-title">配置</h2>
        <p class="qq-sidebar-subtitle">网关服务路径与连接参数</p>
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
        <div class="qq-settings-actions">
          <button class="qq-icon-button" :disabled="disabled" aria-label="恢复默认路径" @click="resetToDefault">
            <RotateCcw class="h-4 w-4" />
          </button>
          <button class="qq-icon-button" :disabled="disabled" aria-label="重启网关服务" @click="restartGateway">
            <Power class="h-4 w-4" />
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
  </section>
</template>
