<script setup>
import { computed, onMounted, ref } from 'vue'
import { RefreshCcw, RotateCcw, Save, Server } from 'lucide-vue-next'
import { useAppStore } from '@/stores/app'

const app = useAppStore()
defineProps({
  section: {
    type: String,
    default: 'gateway',
  },
})
const gatewayPath = ref('')
const savedTip = ref('')

onMounted(async () => {
  await app.loadAppSettings()
  gatewayPath.value = app.gatewayBinaryPath || ''
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
  savedTip.value = ''
  await app.saveAppSettings({ gatewayBinaryPath: gatewayPath.value.trim() })
  await app.refreshGatewayStatus()
  savedTip.value = '已保存'
  setTimeout(() => {
    savedTip.value = ''
  }, 1800)
}

function resetToDefault() {
  gatewayPath.value = './agent-gateway.exe'
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
          placeholder="例如：E:/codes/icoo_ai/agent_gateway/dist/agent-gateway.exe"
        />
        <p class="qq-sidebar-subtitle">
          保存后会写入本地配置，并在当前进程更新网关启动路径（`ICOO_GATEWAY_BIN`）。
        </p>
        <div class="qq-settings-actions">
          <button class="qq-icon-button" :disabled="disabled" aria-label="恢复默认路径" @click="resetToDefault">
            <RotateCcw class="h-4 w-4" />
          </button>
          <button class="qq-icon-button" :disabled="disabled" aria-label="刷新网关状态" @click="app.refreshGatewayStatus">
            <RefreshCcw class="h-4 w-4" />
          </button>
          <button class="qq-primary-action h-9 px-4" :disabled="disabled" @click="saveSettings">
            <Save class="h-4 w-4" />
            <span>{{ app.settingsSaving ? '保存中' : '保存配置' }}</span>
          </button>
          <span v-if="savedTip" class="qq-settings-success">{{ savedTip }}</span>
          <span v-if="app.settingsError" class="qq-settings-error">{{ app.settingsError }}</span>
        </div>
      </div>
    </div>
  </section>
</template>
