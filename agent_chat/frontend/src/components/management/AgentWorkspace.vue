<script setup>
import { computed, onMounted, ref } from 'vue'
import CrudModuleWorkspace from '@/components/management/CrudModuleWorkspace.vue'
import { useAppStore } from '@/stores/app'

const app = useAppStore()
const agents = ref([])
const disabled = computed(() => app.settingsSaving)
const saveDisabled = computed(() => app.settingsSaving || app.settingsLoading || !app.settingsLoaded)

const extraFields = [
  { key: 'protocol', label: '协议', placeholder: 'acp', defaultValue: '' },
  { key: 'description', label: '描述', placeholder: 'Agent 描述', defaultValue: '' },
]

const tableColumns = [
  { key: 'id', label: 'ID' },
  { key: 'name', label: '名称' },
  { key: 'protocol', label: '协议' },
  { key: 'description', label: '描述' },
  { key: 'args', label: '模型', formatter: (item) => (item.args || []).join(' ') },
  { key: 'enabled', label: '启用', type: 'boolean' },
]

const detailFields = [
  { key: 'id', label: 'ID' },
  { key: 'name', label: '名称' },
  { key: 'protocol', label: '协议' },
  { key: 'description', label: '描述' },
  { key: 'args', label: '模型', formatter: (item) => (item.args || []).join(' ') },
  { key: 'enabled', label: '启用', formatter: (item) => (item.enabled ? '是' : '否') },
]

function toViewItems(source = []) {
  return source.map((item) => ({
    id: item.id,
    name: item.name,
    protocol: item.protocol || '',
    description: item.description || '',
    command: '',
    args: Array.isArray(item.models) ? [...item.models] : [],
    enabled: Boolean(item.enabled),
  }))
}

function toStoreItems(source = []) {
  return source.map((item) => ({
    id: item.id,
    name: item.name,
    protocol: item.protocol || '',
    description: item.description || '',
    models: Array.isArray(item.args) ? item.args.map((model) => String(model || '').trim()).filter(Boolean) : [],
    enabled: Boolean(item.enabled),
  }))
}

onMounted(async () => {
  await app.loadAppSettings()
  agents.value = toViewItems(app.agents)
})

function validateAgent(item) {
  if (!item.name) return '请填写名称'
  return ''
}

function onError(message) {
  app.pushToast({ type: 'error', message })
}

async function onRefresh() {
  await app.loadAppSettings()
  agents.value = toViewItems(app.agents)
}

async function onSave(nextItems = null) {
  if (disabled.value) return
  if (Array.isArray(nextItems)) agents.value = nextItems.map((item) => ({ ...item }))
  try {
    await app.saveAppSettings({ agents: toStoreItems(agents.value) })
    agents.value = toViewItems(app.agents)
    app.pushToast({ type: 'success', message: 'Agent 配置保存成功' })
  } catch {
    app.pushToast({ type: 'error', message: app.settingsError || 'Agent 配置保存失败' })
  }
}
</script>

<template>
  <CrudModuleWorkspace
    v-model:items="agents"
    title="Agent 管理"
    subtitle="配置 Agent（id / name / protocol / description / models / enabled）"
    empty-text="暂无 Agent，请先新增。"
    query-placeholder="按 ID / 名称 / 协议 / 模型搜索"
    :extra-fields="extraFields"
    :table-columns="tableColumns"
    :detail-fields="detailFields"
    :loading="app.settingsLoading"
    :error-text="app.settingsError || ''"
    :show-refresh="true"
    :save-disabled="saveDisabled"
    :allow-save="false"
    :persist-on-apply="true"
    :validate-item="validateAgent"
    @error="onError"
    @refresh="onRefresh"
    @save="onSave"
  />
</template>
