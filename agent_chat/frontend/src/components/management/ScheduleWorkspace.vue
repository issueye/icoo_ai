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
  { key: 'content', label: '任务内容', placeholder: '例如：每天 9 点生成昨日审计摘要并发送给管理员', defaultValue: '' },
]

const tableColumns = [
  { key: 'id', label: 'ID' },
  { key: 'name', label: '名称' },
  { key: 'spec', label: 'Cron 表达式' },
  { key: 'content', label: '任务内容' },
  { key: 'enabled', label: '启用', type: 'boolean' },
]

const detailFields = [
  { key: 'id', label: 'ID' },
  { key: 'name', label: '名称' },
  { key: 'spec', label: 'Cron 表达式' },
  { key: 'content', label: '任务内容' },
  { key: 'enabled', label: '启用', formatter: (item) => (item.enabled ? '是' : '否') },
]

function validateTask(item) {
  if (!item.spec) return '请填写 Cron 表达式'
  if (!item.content) return '请填写任务内容'
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
    subtitle="配置任务（id / name / spec / content / enabled）"
    save-label="保存定时任务配置"
    empty-text="暂无定时任务，请先新增。"
    :extra-fields="extraFields"
    :table-columns="tableColumns"
    :detail-fields="detailFields"
    :include-command-args="false"
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
