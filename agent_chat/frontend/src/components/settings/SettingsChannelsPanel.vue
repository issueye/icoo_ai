<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { PenSquare, Plus, Trash2 } from 'lucide-vue-next'
import ContextDropdown from '@/components/ui/ContextDropdown.vue'
import { useAppStore } from '@/stores/app'

const app = useAppStore()

const channelTypeOptions = [
  { id: 'qq', label: 'QQ 机器人' },
  { id: 'lark', label: '飞书机器人' },
  { id: 'wechat', label: '微信机器人' },
]

const channelMetaMap = {
  qq: {
    name: 'QQ 机器人',
    subtitle: '用于 QQ 群与私聊消息收发',
    placeholders: {
      appId: '例如：qq-app-id',
      appSecret: '例如：qq-app-secret',
      botToken: '例如：qq-bot-token',
      webhookUrl: '例如：https://qq.example.com/webhook',
    },
  },
  lark: {
    name: '飞书机器人',
    subtitle: '用于飞书群通知与指令回调',
    placeholders: {
      appId: '例如：cli_a1b2c3',
      appSecret: '例如：lark-app-secret',
      botToken: '例如：lark-bot-token',
      webhookUrl: '例如：https://open.feishu.cn/open-apis/bot/v2/hook/xxxx',
    },
  },
  wechat: {
    name: '微信机器人',
    subtitle: '用于企业微信机器人消息分发',
    placeholders: {
      appId: '例如：wx-app-id',
      appSecret: '例如：wx-app-secret',
      botToken: '例如：wechat-bot-token',
      webhookUrl: '例如：https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=xxxx',
    },
  },
}

const supportedChannelTypes = channelTypeOptions.map((item) => item.id)
const channelDrafts = ref([])
const editorOpen = ref(false)
const editorMode = ref('edit')
const editorIndex = ref(-1)
const editorDraft = ref(null)

function normalizeChannelType(rawType, fallbackType = 'qq') {
  const source = typeof rawType === 'string' ? rawType.trim().toLowerCase() : ''
  if (supportedChannelTypes.includes(source)) return source
  const fallback = typeof fallbackType === 'string' ? fallbackType.trim().toLowerCase() : ''
  if (supportedChannelTypes.includes(fallback)) return fallback
  return 'qq'
}

function channelMeta(type) {
  const normalizedType = normalizeChannelType(type)
  return channelMetaMap[normalizedType] || channelMetaMap.qq
}

function defaultChannelID(type, sequence = 1) {
  const normalizedType = normalizeChannelType(type)
  return sequence > 1 ? `${normalizedType}_${sequence}` : normalizedType
}

function createDefaultChannel(type = 'qq', sequence = 1) {
  const normalizedType = normalizeChannelType(type)
  const meta = channelMeta(normalizedType)
  return {
    id: defaultChannelID(normalizedType, sequence),
    name: meta.name,
    type: normalizedType,
    enabled: false,
    appId: '',
    appSecret: '',
    botToken: '',
    webhookUrl: '',
  }
}

function normalizeChannel(rawChannel = {}, fallbackType = 'qq') {
  const type = normalizeChannelType(rawChannel?.type, fallbackType)
  const defaults = createDefaultChannel(type, 1)
  const normalizedID = typeof rawChannel?.id === 'string' ? rawChannel.id.trim() : ''
  const normalizedName = typeof rawChannel?.name === 'string' ? rawChannel.name.trim() : ''
  const normalizedAppID = typeof rawChannel?.appId === 'string' ? rawChannel.appId.trim() : ''
  const normalizedAppSecret = typeof rawChannel?.appSecret === 'string' ? rawChannel.appSecret.trim() : ''
  const normalizedBotToken = typeof rawChannel?.botToken === 'string' ? rawChannel.botToken.trim() : ''
  const normalizedWebhookURL = typeof rawChannel?.webhookUrl === 'string' ? rawChannel.webhookUrl.trim() : ''
  return {
    id: normalizedID || defaults.id,
    name: normalizedName || defaults.name,
    type,
    enabled: Boolean(rawChannel?.enabled),
    appId: normalizedAppID,
    appSecret: normalizedAppSecret,
    botToken: normalizedBotToken,
    webhookUrl: normalizedWebhookURL,
  }
}

function ensureUniqueChannelIDs(channels = []) {
  const used = new Set()
  const typeCounters = new Map()
  return channels.map((channel) => {
    const type = normalizeChannelType(channel?.type)
    const nextTypeCount = (typeCounters.get(type) || 0) + 1
    typeCounters.set(type, nextTypeCount)
    const requestedID = String(channel?.id || '').trim()
    let id = requestedID || defaultChannelID(type, nextTypeCount)
    if (!used.has(id)) {
      used.add(id)
      return { ...channel, id, type }
    }
    let suffix = nextTypeCount
    while (used.has(`${id}_${suffix}`)) suffix += 1
    id = `${id}_${suffix}`
    used.add(id)
    return { ...channel, id, type }
  })
}

function normalizeChannels(rawChannels) {
  const source = Array.isArray(rawChannels) ? rawChannels : []
  if (source.length === 0) {
    return []
  }
  const normalized = source.map((rawChannel, index) => {
    const fallbackType = supportedChannelTypes[index % supportedChannelTypes.length] || 'qq'
    return normalizeChannel(rawChannel, fallbackType)
  })
  return ensureUniqueChannelIDs(normalized)
}

function syncChannelsFromStore() {
  channelDrafts.value = normalizeChannels(app.channels).map((channel) => ({ ...channel }))
}

onMounted(syncChannelsFromStore)

watch(
  () => app.channels,
  () => syncChannelsFromStore(),
  { deep: true },
)

const disabled = computed(() => app.settingsSaving)

function summarizeText(value) {
  const text = String(value || '').trim()
  if (!text) return '未配置'
  if (text.length <= 26) return text
  return `${text.slice(0, 10)}...${text.slice(-10)}`
}

function hideSecret(value) {
  const text = String(value || '').trim()
  if (!text) return '未配置'
  if (text.length <= 10) return '******'
  return `${text.slice(0, 3)}******${text.slice(-3)}`
}

function openCreateEditor() {
  editorMode.value = 'create'
  editorIndex.value = -1
  editorDraft.value = createDefaultChannel('qq', channelDrafts.value.length + 1)
  editorOpen.value = true
}

function openEditEditor(channel, index) {
  if (!channel || typeof channel !== 'object') return
  editorMode.value = 'edit'
  editorIndex.value = Number(index)
  editorDraft.value = { ...normalizeChannel(channel, channel.type) }
  editorOpen.value = true
}

function closeEditor() {
  editorOpen.value = false
  editorMode.value = 'edit'
  editorIndex.value = -1
  editorDraft.value = null
}

async function persistChannels(successMessage) {
  try {
    await app.saveAppSettings({
      channels: normalizeChannels(channelDrafts.value),
    })
    if (successMessage) {
      app.pushToast({ type: 'success', message: successMessage })
    }
    return true
  } catch {
    app.pushToast({ type: 'error', message: app.settingsError || '渠道配置保存失败' })
    return false
  }
}

async function toggleChannel(channel) {
  if (!channel || typeof channel !== 'object') return
  channel.enabled = !channel.enabled
  const success = await persistChannels(channel.enabled ? `${channel.name} 已启用` : `${channel.name} 已禁用`)
  if (!success) channel.enabled = !channel.enabled
}

async function removeChannel(index) {
  const removed = channelDrafts.value[index]
  const backup = channelDrafts.value.map((channel) => ({ ...channel }))
  channelDrafts.value.splice(index, 1)
  const success = await persistChannels(`${removed?.name || '渠道'} 已删除`)
  if (!success) {
    channelDrafts.value = backup
  }
}

async function saveChannelDetail() {
  if (!editorDraft.value) {
    closeEditor()
    return
  }
  const nextChannels = channelDrafts.value.map((channel) => ({ ...channel }))
  const normalizedDraft = normalizeChannel(editorDraft.value, editorDraft.value.type)
  if (editorMode.value === 'create') {
    nextChannels.push(normalizedDraft)
  } else if (editorIndex.value >= 0 && editorIndex.value < nextChannels.length) {
    nextChannels[editorIndex.value] = normalizedDraft
  } else {
    closeEditor()
    return
  }

  const backup = channelDrafts.value.map((channel) => ({ ...channel }))
  channelDrafts.value = normalizeChannels(nextChannels)
  const actionLabel = editorMode.value === 'create' ? '已新增' : '配置已更新'
  const success = await persistChannels(`${normalizedDraft.name} ${actionLabel}`)
  if (!success) {
    channelDrafts.value = backup
    return
  }
  closeEditor()
}
</script>

<template>
  <section class="qq-chat-workspace">
    <header class="qq-chat-header qq-settings-header">
      <div class="min-w-0 flex-1">
        <h2 class="qq-chat-title">渠道管理</h2>
        <p class="qq-sidebar-subtitle">QQ / 飞书 / 微信 机器人接入</p>
      </div>
      <button class="qq-primary-action h-8 px-3 text-sm" :disabled="disabled" @click="openCreateEditor">
        <Plus class="h-4 w-4" />
        <span>新增渠道</span>
      </button>
    </header>

    <div class="qq-channel-layout">
      <div class="qq-channel-intro">
        <h3 class="qq-settings-card-title">机器人渠道配置</h3>
        <p class="qq-settings-helper">支持动态新增多个渠道实例，例如多个 QQ、多个飞书机器人。</p>
      </div>
      <div class="qq-channel-grid">
        <article
          v-for="(channel, index) in channelDrafts"
          :key="channel.id"
          class="qq-channel-card"
          :class="{ 'is-enabled': channel.enabled }"
        >
          <div class="qq-channel-head">
            <div class="min-w-0">
              <div class="qq-channel-title">{{ channel.name }}</div>
              <div class="qq-channel-subtitle">{{ channelMeta(channel.type).subtitle }}</div>
            </div>
            <button
              type="button"
              class="qq-channel-toggle"
              :class="{ 'is-enabled': channel.enabled }"
              :disabled="disabled"
              @click="toggleChannel(channel)"
            >
              <span class="qq-channel-toggle-dot" />
              <span>{{ channel.enabled ? '已启用' : '未启用' }}</span>
            </button>
          </div>
          <div class="qq-channel-meta">
            <div class="qq-channel-meta-row">
              <span class="qq-channel-meta-label">渠道类型</span>
              <span class="qq-channel-meta-value">{{ channel.type }}</span>
            </div>
            <div class="qq-channel-meta-row">
              <span class="qq-channel-meta-label">App ID</span>
              <span class="qq-channel-meta-value">{{ summarizeText(channel.appId) }}</span>
            </div>
            <div class="qq-channel-meta-row">
              <span class="qq-channel-meta-label">App Secret</span>
              <span class="qq-channel-meta-value">{{ hideSecret(channel.appSecret) }}</span>
            </div>
            <div class="qq-channel-meta-row">
              <span class="qq-channel-meta-label">Bot Token</span>
              <span class="qq-channel-meta-value">{{ hideSecret(channel.botToken) }}</span>
            </div>
            <div class="qq-channel-meta-row">
              <span class="qq-channel-meta-label">Webhook</span>
              <span class="qq-channel-meta-value">{{ summarizeText(channel.webhookUrl) }}</span>
            </div>
          </div>
          <div class="qq-channel-actions">
            <button class="qq-secondary-action h-8 px-3 text-sm qq-channel-edit" :disabled="disabled" @click="openEditEditor(channel, index)">
              <PenSquare class="h-4 w-4" />
              <span>编辑明细</span>
            </button>
            <button class="qq-secondary-action h-8 px-3 text-sm qq-channel-delete" :disabled="disabled" @click="removeChannel(index)">
              <Trash2 class="h-4 w-4" />
              <span>删除</span>
            </button>
          </div>
        </article>
      </div>
      <p v-if="app.settingsError" class="qq-settings-error">{{ app.settingsError }}</p>
    </div>

    <div v-if="editorOpen && editorDraft" class="qq-modal-backdrop" @click.self="closeEditor">
      <div class="qq-modal" role="dialog" aria-modal="true" aria-labelledby="channelEditorTitle">
        <div class="qq-modal-header">
          <h3 id="channelEditorTitle" class="qq-modal-title">
            {{ editorMode === 'create' ? '新增渠道' : `${editorDraft.name} 配置明细` }}
          </h3>
          <button class="qq-icon-button" type="button" aria-label="关闭弹窗" @click="closeEditor">×</button>
        </div>
        <div class="qq-modal-body">
          <label class="qq-modal-field">
            <span class="qq-modal-label">渠道类型</span>
            <ContextDropdown v-model="editorDraft.type" class="qq-settings-dropdown" label="类型" :options="channelTypeOptions" />
          </label>
          <label class="qq-modal-field">
            <span class="qq-modal-label">渠道名称</span>
            <input v-model="editorDraft.name" type="text" class="qq-settings-input" placeholder="例如：QQ 机器人（业务群）" />
          </label>
          <label class="qq-modal-field">
            <span class="qq-modal-label">App ID</span>
            <input v-model="editorDraft.appId" type="text" class="qq-settings-input" :placeholder="channelMeta(editorDraft.type).placeholders.appId" />
          </label>
          <label class="qq-modal-field">
            <span class="qq-modal-label">App Secret</span>
            <input v-model="editorDraft.appSecret" type="text" class="qq-settings-input" :placeholder="channelMeta(editorDraft.type).placeholders.appSecret" />
          </label>
          <label class="qq-modal-field">
            <span class="qq-modal-label">Bot Token</span>
            <input v-model="editorDraft.botToken" type="text" class="qq-settings-input" :placeholder="channelMeta(editorDraft.type).placeholders.botToken" />
          </label>
          <label class="qq-modal-field">
            <span class="qq-modal-label">Webhook URL</span>
            <input v-model="editorDraft.webhookUrl" type="text" class="qq-settings-input" :placeholder="channelMeta(editorDraft.type).placeholders.webhookUrl" />
          </label>
        </div>
        <div class="qq-modal-actions">
          <button class="qq-secondary-action h-8 px-3 text-sm" type="button" :disabled="disabled" @click="closeEditor">取消</button>
          <button class="qq-primary-action h-8 px-3 text-sm" type="button" :disabled="disabled" @click="saveChannelDetail">
            {{ app.settingsSaving ? '保存中' : (editorMode === 'create' ? '新增渠道' : '保存明细') }}
          </button>
        </div>
      </div>
    </div>
  </section>
</template>
