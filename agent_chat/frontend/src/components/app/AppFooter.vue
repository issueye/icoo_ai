<script setup>
import { computed } from 'vue'
import { useAppStore } from '@/stores/app'

const app = useAppStore()

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
</script>

<template>
  <footer class="qq-global-footer">
    <div class="qq-footer-left">
      <span class="qq-footer-label">网关服务</span>
      <span class="qq-footer-status" :class="statusClass">
        <span class="qq-footer-dot" />
        <span>{{ statusText }}</span>
      </span>
    </div>
    <div class="qq-footer-summary">
      {{ app.gatewaySummary || '等待状态更新' }}
    </div>
  </footer>
</template>

