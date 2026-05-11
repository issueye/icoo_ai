<script setup>
import { computed, ref } from 'vue'
import { useAppStore } from '@/stores/app'

const app = useAppStore()
const reconnecting = ref(false)

const statusText = computed(() => {
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

const canReconnect = computed(() => app.gatewayStatus === 'gateway_failed')

async function reconnectGateway() {
  if (!canReconnect.value || reconnecting.value) return
  reconnecting.value = true
  try {
    const reconnectAction = typeof app.reconnectGateway === 'function'
      ? app.reconnectGateway
      : app.restartGateway
    await reconnectAction()
    app.pushToast({ type: 'success', message: '网关重新连接成功' })
  } catch (error) {
    app.pushToast({ type: 'error', message: error?.message || '网关重新连接失败' })
  } finally {
    reconnecting.value = false
  }
}
</script>

<template>
  <footer class="qq-global-footer">
    <div class="qq-footer-left">
      <span class="qq-footer-label">网关服务</span>
      <span class="qq-footer-status" :class="statusClass">
        <span class="qq-footer-dot" />
        <span>{{ statusText }}</span>
      </span>
      <button
        v-if="canReconnect"
        class="qq-footer-reconnect"
        type="button"
        :disabled="reconnecting"
        @click="reconnectGateway"
      >
        {{ reconnecting ? '重连中…' : '重新连接' }}
      </button>
    </div>
    <div class="qq-footer-summary">
      {{ app.gatewaySummary || '等待状态更新' }}
    </div>
  </footer>
</template>
