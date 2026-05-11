<script setup>
import { computed, onMounted, ref } from 'vue'
import CrudModuleWorkspace from '@/components/management/CrudModuleWorkspace.vue'
import { useAppStore } from '@/stores/app'

const app = useAppStore()
const servers = ref([])
const disabled = computed(() => app.settingsSaving)
const saveDisabled = computed(() => app.settingsSaving || app.settingsLoading || !app.settingsLoaded)

const tableColumns = [
  { key: 'id', label: 'ID' },
  { key: 'name', label: '名称' },
  { key: 'command', label: '命令' },
  { key: 'args', label: '参数', formatter: (item) => (item.args || []).join(' ') },
  { key: 'enabled', label: '启用', type: 'boolean' },
]

const detailFields = [
  { key: 'id', label: 'ID' },
  { key: 'name', label: '名称' },
  { key: 'command', label: '命令' },
  { key: 'args', label: '参数', formatter: (item) => (item.args || []).join(' ') },
  { key: 'enabled', label: '启用', formatter: (item) => (item.enabled ? '是' : '否') },
]

onMounted(async () => {
  await app.loadAppSettings()
  servers.value = Array.isArray(app.mcpServers) ? app.mcpServers.map((item) => ({ ...item })) : []
})

function validateServer(item) {
  if (!item.command) return '请填写 command'
  return ''
}

function onError(message) {
  app.pushToast({ type: 'error', message })
}

async function onRefresh() {
  await app.loadAppSettings()
  servers.value = Array.isArray(app.mcpServers) ? app.mcpServers.map((item) => ({ ...item })) : []
}

async function onSave(nextItems = null) {
  if (disabled.value) return
  if (Array.isArray(nextItems)) servers.value = nextItems.map((item) => ({ ...item }))
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
    :table-columns="tableColumns"
    :detail-fields="detailFields"
    :loading="app.settingsLoading"
    :error-text="app.settingsError || ''"
    :show-refresh="true"
    :save-disabled="saveDisabled"
    :allow-save="false"
    :persist-on-apply="true"
    :validate-item="validateServer"
    @error="onError"
    @refresh="onRefresh"
    @save="onSave"
  />
</template>
