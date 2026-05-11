<script setup>
import { computed, onMounted, ref } from 'vue'
import CrudModuleWorkspace from '@/components/management/CrudModuleWorkspace.vue'
import { useAppStore } from '@/stores/app'

const app = useAppStore()
const tasks = ref([])
const disabled = computed(() => app.settingsSaving)

onMounted(async () => {
  await app.loadAppSettings()
  tasks.value = Array.isArray(app.scheduleTasks) ? app.scheduleTasks.map((item) => ({ ...item })) : []
})

const extraFields = [
  { key: 'spec', label: 'Cron 表达式', placeholder: '*/5 * * * *', defaultValue: '*/5 * * * *' },
]

function validateTask(item) {
  if (!item.command) return '请填写 command'
  if (!item.spec) return '请填写 Cron 表达式'
  return ''
}

function formatSubtitle(item) {
  return `${item.spec || ''} · ${item.command || ''} ${(item.args || []).join(' ')}`.trim()
}

function onError(message) {
  app.pushToast({ type: 'error', message })
}

async function onSave() {
  if (disabled.value) return
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
    :validate-item="validateTask"
    :format-subtitle="formatSubtitle"
    @error="onError"
    @save="onSave"
  />
</template>
