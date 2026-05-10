<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { Save } from 'lucide-vue-next'
import { useAppStore } from '@/stores/app'

const app = useAppStore()

const channelOrder = ['qq', 'lark', 'wechat']
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
const channelDrafts = ref([])

function normalizeChannelType(rawType, fallbackType = 'qq') {
  const source = typeof rawType === 'string' ? rawType.trim().toLowerCase() : ''
  if (channelOrder.includes(source)) return source
  const fallback = typeof fallbackType === 'string' ? fallbackType.trim().toLowerCase() : ''
  if (channelOrder.includes(fallback)) return fallback
  return 'qq'
}

function channelMeta(type) {
  const normalizedType = normalizeChannelType(type)
  return channelMetaMap[normalizedType] || channelMetaMap.qq
}

function normalizeChannel(rawChannel = {}, fallbackType = 'qq') {
  const type = normalizeChannelType(rawChannel?.type, fallbackType)
  const meta = channelMeta(type)
  const normalizedID = typeof rawChannel?.id === 'string' ? rawChannel.id.trim() : ''
  const normalizedName = typeof rawChannel?.name === 'string' ? rawChannel.name.trim() : ''
  const normalizedAppID = typeof rawChannel?.appId === 'string' ? rawChannel.appId.trim() : ''
  const normalizedAppSecret = typeof rawChannel?.appSecret === 'string' ? rawChannel.appSecret.trim() : ''
  const normalizedBotToken = typeof rawChannel?.botToken === 'string' ? rawChannel.botToken.trim() : ''
  const normalizedWebhookURL = typeof rawChannel?.webhookUrl === 'string' ? rawChannel.webhookUrl.trim() : ''
  return {
    id: normalizedID || type,
    name: normalizedName || meta.name,
    type,
    enabled: Boolean(rawChannel?.enabled),
    appId: normalizedAppID,
    appSecret: normalizedAppSecret,
    botToken: normalizedBotToken,
    webhookUrl: normalizedWebhookURL,
  }
}

function normalizeChannels(rawChannels) {
  const source = Array.isArray(rawChannels) ? rawChannels : []
  const channelsByType = new Map()
  source.forEach((rawChannel, index) => {
    const fallbackType = channelOrder[index] || 'qq'
    const normalized = normalizeChannel(rawChannel, fallbackType)
    if (!channelsByType.has(normalized.type)) {
      channelsByType.set(normalized.type, normalized)
    }
  })
  return channelOrder.map((type) => channelsByType.get(type) || normalizeChannel({ type }, type))
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

function toggleChannel(channel) {
  if (!channel || typeof channel !== 'object') return
  channel.enabled = !channel.enabled
}

async function saveChannels() {
  try {
    await app.saveAppSettings({
      channels: normalizeChannels(channelDrafts.value),
    })
    app.pushToast({ type: 'success', message: '渠道配置保存成功' })
  } catch {
    app.pushToast({ type: 'error', message: app.settingsError || '渠道配置保存失败' })
  }
}
</script>

<template>
  <section class="qq-chat-workspace">
    <header class="qq-chat-header qq-settings-header">
      <div class="min-w-0 flex-1">
        <h2 class="qq-chat-title">渠道管理</h2>
        <p class="qq-sidebar-subtitle">QQ / 飞书 / 微信 机器人接入</p>
      </div>
    </header>

    <div class="qq-settings-body">
      <div class="qq-settings-card">
        <h3 class="qq-settings-card-title">机器人渠道配置</h3>
        <p class="qq-settings-helper">为每个渠道填写凭据与回调地址，启用后即可接入消息链路。</p>
        <div class="qq-channel-grid">
          <article
            v-for="channel in channelDrafts"
            :key="channel.type"
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

            <div class="qq-channel-fields">
              <div class="qq-channel-field">
                <label class="qq-settings-label" :for="`${channel.type}_appId`">App ID</label>
                <input
                  :id="`${channel.type}_appId`"
                  v-model="channel.appId"
                  type="text"
                  class="qq-settings-input"
                  :placeholder="channelMeta(channel.type).placeholders.appId"
                />
              </div>

              <div class="qq-channel-field">
                <label class="qq-settings-label" :for="`${channel.type}_appSecret`">App Secret</label>
                <input
                  :id="`${channel.type}_appSecret`"
                  v-model="channel.appSecret"
                  type="text"
                  class="qq-settings-input"
                  :placeholder="channelMeta(channel.type).placeholders.appSecret"
                />
              </div>

              <div class="qq-channel-field">
                <label class="qq-settings-label" :for="`${channel.type}_botToken`">Bot Token</label>
                <input
                  :id="`${channel.type}_botToken`"
                  v-model="channel.botToken"
                  type="text"
                  class="qq-settings-input"
                  :placeholder="channelMeta(channel.type).placeholders.botToken"
                />
              </div>

              <div class="qq-channel-field is-wide">
                <label class="qq-settings-label" :for="`${channel.type}_webhook`">Webhook URL</label>
                <input
                  :id="`${channel.type}_webhook`"
                  v-model="channel.webhookUrl"
                  type="text"
                  class="qq-settings-input"
                  :placeholder="channelMeta(channel.type).placeholders.webhookUrl"
                />
              </div>
            </div>
          </article>
        </div>
        <div class="qq-settings-actions">
          <button class="qq-primary-action h-9 px-4" :disabled="disabled" @click="saveChannels">
            <Save class="h-4 w-4" />
            <span>{{ app.settingsSaving ? '保存中' : '保存渠道配置' }}</span>
          </button>
          <span v-if="app.settingsError" class="qq-settings-error">{{ app.settingsError }}</span>
        </div>
      </div>
    </div>
  </section>
</template>
