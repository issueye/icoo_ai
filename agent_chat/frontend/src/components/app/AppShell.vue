<script setup>
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import AppGlobalHeader from './AppGlobalHeader.vue'
import AppNavRail from './AppNavRail.vue'
import AppFooter from './AppFooter.vue'
import AppToastStack from './AppToastStack.vue'
import ConversationSidebar from '@/components/conversation/ConversationSidebar.vue'
import ChatWorkspace from '@/components/chat/ChatWorkspace.vue'
import AuditWorkspace from '@/components/audit/AuditWorkspace.vue'
import SkillsWorkspace from '@/components/skills/SkillsWorkspace.vue'
import AgentWorkspace from '@/components/management/AgentWorkspace.vue'
import McpWorkspace from '@/components/management/McpWorkspace.vue'
import ScheduleWorkspace from '@/components/management/ScheduleWorkspace.vue'
import ChannelWorkspace from '@/components/management/ChannelWorkspace.vue'
import SettingsSidebar from '@/components/settings/SettingsSidebar.vue'
import SettingsWorkspace from '@/components/settings/SettingsWorkspace.vue'
import { useApprovalsStore } from '@/stores/approvals'
import { useAppStore } from '@/stores/app'
import { useAuditStore } from '@/stores/audit'
import { useConversationsStore } from '@/stores/conversations'
import { useMessagesStore } from '@/stores/messages'
import { useRunsStore } from '@/stores/runs'
import { useSkillsStore } from '@/stores/skills'
import { subscribeAgentEvents } from '@/services/agentEvents'

const route = useRoute()
const router = useRouter()
const app = useAppStore()
const conversations = useConversationsStore()
const messages = useMessagesStore()
const runs = useRunsStore()
const approvals = useApprovalsStore()
const skills = useSkillsStore()
const audit = useAuditStore()
let statusTimer = null
let unsubscribeAgentEvents = null
const isAuditRoute = () => route.name === 'audit'
const isChatRoute = () => route.name === 'chats' || route.name === 'chat'
const activeSettingsSection = ref('gateway')

onMounted(async () => {
  unsubscribeAgentEvents = subscribeAgentEvents({
    app,
    messages,
    runs,
    approvals,
    audit,
  })
  await app.refreshGatewayStatus()
  statusTimer = setInterval(() => {
    app.refreshGatewayStatus()
  }, 3000)
  await Promise.all([
    app.loadAppSettings(),
    conversations.loadAgentProfiles(),
    conversations.loadConversations(),
    runs.loadRuns(),
    approvals.loadApprovals(),
    skills.loadSkills(),
    audit.loadAuditEvents(),
  ])
  const routeSessionId = route.params.sessionId
  if (isChatRoute() && routeSessionId) conversations.setActiveSession(String(routeSessionId))
  if (isChatRoute() && conversations.activeSessionId) {
    await messages.loadMessages(conversations.activeSessionId)
    if (!routeSessionId) router.replace(`/chats/${conversations.activeSessionId}`)
  }
  if (isAuditRoute()) {
    audit.markViewed()
  }
})

onBeforeUnmount(() => {
  if (typeof unsubscribeAgentEvents === 'function') {
    unsubscribeAgentEvents()
    unsubscribeAgentEvents = null
  }
  if (statusTimer) {
    clearInterval(statusTimer)
    statusTimer = null
  }
})

watch(() => route.params.sessionId, async (sessionId) => {
  if (!isChatRoute()) return
  if (!sessionId) return
  conversations.setActiveSession(String(sessionId))
  await messages.loadMessages(String(sessionId))
})

watch(() => route.name, (name) => {
  if (name === 'settings') app.setActiveNav('settings')
  else if (name === 'audit') app.setActiveNav('audit')
  else if (name === 'channels') app.setActiveNav('channels')
  else if (name === 'skills') app.setActiveNav('skills')
  else if (name === 'agents') app.setActiveNav('agents')
  else if (name === 'mcp') app.setActiveNav('mcp')
  else if (name === 'schedule') app.setActiveNav('schedule')
  else app.setActiveNav('chats')
  if (name === 'audit') {
    audit.markViewed()
  }
}, { immediate: true })

watch(() => conversations.activeSessionId, async (sessionId) => {
  if (!isChatRoute()) return
  if (!sessionId) return
  if (route.params.sessionId !== sessionId) router.push(`/chats/${sessionId}`)
  await messages.loadMessages(sessionId)
})
</script>

<template>
  <main class="qq-window-shell">
    <AppGlobalHeader />
    <div class="qq-shell">
      <AppNavRail />
      <template v-if="route.name === 'settings'">
        <SettingsSidebar v-model="activeSettingsSection" />
        <SettingsWorkspace :section="activeSettingsSection" />
      </template>
      <template v-else-if="route.name === 'audit'">
        <AuditWorkspace />
      </template>
      <template v-else-if="route.name === 'channels'">
        <ChannelWorkspace />
      </template>
      <template v-else-if="route.name === 'skills'">
        <SkillsWorkspace />
      </template>
      <template v-else-if="route.name === 'agents'">
        <AgentWorkspace />
      </template>
      <template v-else-if="route.name === 'mcp'">
        <McpWorkspace />
      </template>
      <template v-else-if="route.name === 'schedule'">
        <ScheduleWorkspace />
      </template>
      <template v-else>
        <ConversationSidebar />
        <ChatWorkspace />
      </template>
    </div>
    <AppFooter />
    <AppToastStack />
  </main>
</template>
