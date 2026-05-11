<script setup>
import { computed, onMounted, ref } from 'vue'
import CrudModuleWorkspace from '@/components/management/CrudModuleWorkspace.vue'
import { useAppStore } from '@/stores/app'

const app = useAppStore()
const servers = ref([])
const disabled = computed(() => app.settingsSaving)

onMounted(async () => {
  await app.loadAppSettings()
  servers.value = Array.isArray(app.mcpServers) ? app.mcpServers.map((item) => ({ ...item })) : []
})

function validateServer(item) {
  if (!item.command) return '请填写 command'
  return ''
}

function formatSubtitle(item) {
  return `${item.command || ''} ${(item.args || []).join(' ')}`.trim()
}

function onError(message) {
  app.pushToast({ type: 'error', message })
}

async function onSave() {
  if (disabled.value) return
  try {
    await app.saveAppSettings({ mcpServers: servers.value })
    servers.value = Array.isArray(app.mcpServers) ? app.mcpServers.map((item) => ({ ...item })) : []
    app.pushToast({ type: 'success', message: 'MCP 配置保存成功' })
  } catch {
    app.pushToast({ type: 'error', message: app.settingsError || 'MCP 配置保存失败' })
  }
}
</script>

<template>
  <CrudModuleWorkspace
    v-model:items="servers"
    title="MCP 管理"
    subtitle="配置 MCP servers（id / name / command / args / enabled）"
    save-label="保存 MCP 配置"
    empty-text="暂无 MCP server，请先新增。"
    :validate-item="validateServer"
    :format-subtitle="formatSubtitle"
    @error="onError"
    @save="onSave"
  />
</template>
