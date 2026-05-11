<script setup>
import { computed, onMounted, ref } from 'vue'
import CrudModuleWorkspace from '@/components/management/CrudModuleWorkspace.vue'
import { useAppStore } from '@/stores/app'

const app = useAppStore()
const tasks = ref([])
const disabled = computed(() => app.settingsSaving)
const saveDisabled = computed(() => app.settingsSaving || app.settingsLoading || !app.settingsLoaded)

onMounted(async () => {
  await app.loadAppSettings()
  tasks.value = Array.isArray(app.scheduleTasks) ? app.scheduleTasks.map((item) => ({ ...item })) : []
})

const extraFields = [
  { key: 'spec', label: 'Cron 表达式', placeholder: '*/5 * * * *', defaultValue: '*/5 * * * *' },
]

const tableColumns = [
  { key: 'id', label: 'ID' },
  { key: 'name', label: '名称' },
  { key: 'spec', label: 'Cron 表达式' },
  { key: 'command', label: '命令' },
  { key: 'args', label: '参数', formatter: (item) => (item.args || []).join(' ') },
  { key: 'enabled', label: '启用', type: 'boolean' },
]

const detailFields = [
  { key: 'id', label: 'ID' },
  { key: 'name', label: '名称' },
  { key: 'spec', label: 'Cron 表达式' },
  { key: 'command', label: '命令' },
  { key: 'args', label: '参数', formatter: (item) => (item.args || []).join(' ') },
  { key: 'enabled', label: '启用', formatter: (item) => (item.enabled ? '是' : '否') },
]

function validateTask(item) {
  if (!item.command) return '请填写 command'
  if (!item.spec) return '请填写 Cron 表达式'
  return ''
}

function onError(message) {
  app.pushToast({ type: 'error', message })
}

async function onRefresh() {
  await app.loadAppSettings()
  tasks.value = Array.isArray(app.scheduleTasks) ? app.scheduleTasks.map((item) => ({ ...item })) : []
}

async function onSave(nextItems = null) {
  if (disabled.value) return
  if (Array.isArray(nextItems)) tasks.value = nextItems.map((item) => ({ ...item }))
  try {
    await app.saveAppSettings({ scheduleTasks: tasks.value })
    tasks.value = Array.isArray(app.scheduleTasks) ? app.scheduleTasks.map((item) => ({ ...item })) : []
    app.pushToast({ type: 'success', message: '定时任务配置保存成功' })
  } catch {
    app.pushToast({ type: 'error', message: app.settingsError || '定时任务配置保存失败' })
  }
}
</script>

<template>
  <CrudModuleWorkspace
    v-model:items="tasks"
    title="定时任务管理"
    subtitle="配置任务（id / name / spec / command / args / enabled）"
    save-label="保存定时任务配置"
    empty-text="暂无定时任务，请先新增。"
    :extra-fields="extraFields"
    :table-columns="tableColumns"
    :detail-fields="detailFields"
    :loading="app.settingsLoading"
    :error-text="app.settingsError || ''"
    :show-refresh="true"
    :save-disabled="saveDisabled"
    :allow-save="false"
    :persist-on-apply="true"
    :validate-item="validateTask"
    @error="onError"
    @refresh="onRefresh"
    @save="onSave"
  />
</template>
