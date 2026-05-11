<script setup>
import { computed, onMounted, ref } from 'vue'
import CrudModuleWorkspace from '@/components/management/CrudModuleWorkspace.vue'
import { useAppStore } from '@/stores/app'

const app = useAppStore()
const channels = ref([])
const disabled = computed(() => app.settingsSaving)
const saveDisabled = computed(() => app.settingsSaving || app.settingsLoading || !app.settingsLoaded)

const extraFields = [
  { key: 'type', label: '渠道类型', placeholder: 'qq / lark / wechat', defaultValue: 'qq' },
  { key: 'appId', label: 'App ID', placeholder: '应用 ID', defaultValue: '' },
  { key: 'appSecret', label: 'App Secret', placeholder: '应用密钥', defaultValue: '' },
  { key: 'botToken', label: 'Bot Token', placeholder: '机器人 Token', defaultValue: '' },
  { key: 'webhookUrl', label: 'Webhook URL', placeholder: 'https://...', defaultValue: '' },
]

const tableColumns = [
  { key: 'id', label: 'ID' },
  { key: 'name', label: '名称' },
  { key: 'type', label: '渠道类型' },
  { key: 'appId', label: 'App ID' },
  { key: 'enabled', label: '启用', type: 'boolean' },
]

const detailFields = [
  { key: 'id', label: 'ID' },
  { key: 'name', label: '名称' },
  { key: 'type', label: '渠道类型' },
  { key: 'appId', label: 'App ID' },
  { key: 'appSecret', label: 'App Secret' },
  { key: 'botToken', label: 'Bot Token' },
  { key: 'webhookUrl', label: 'Webhook URL' },
  { key: 'enabled', label: '启用', formatter: (item) => (item.enabled ? '是' : '否') },
]

onMounted(async () => {
  await app.loadAppSettings()
  channels.value = Array.isArray(app.channels) ? app.channels.map((item) => ({ ...item })) : []
})

function validateChannel(item) {
  if (!item.type) return '请填写渠道类型'
  return ''
}

function onError(message) {
  app.pushToast({ type: 'error', message })
}

async function onRefresh() {
  await app.loadAppSettings()
  channels.value = Array.isArray(app.channels) ? app.channels.map((item) => ({ ...item })) : []
}

async function onSave(nextItems = null) {
  if (disabled.value) return
  if (Array.isArray(nextItems)) channels.value = nextItems.map((item) => ({ ...item }))
  try {
    await app.saveAppSettings({ channels: channels.value })
    channels.value = Array.isArray(app.channels) ? app.channels.map((item) => ({ ...item })) : []
    app.pushToast({ type: 'success', message: '渠道配置保存成功' })
  } catch {
    app.pushToast({ type: 'error', message: app.settingsError || '渠道配置保存失败' })
  }
}
</script>

<template>
  <CrudModuleWorkspace
    v-model:items="channels"
    title="渠道管理"
    subtitle="配置渠道（id / name / type / 凭据 / enabled）"
    save-label="保存渠道配置"
    empty-text="暂无渠道，请先新增。"
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
    :validate-item="validateChannel"
    @error="onError"
    @refresh="onRefresh"
    @save="onSave"
  />
</template>
