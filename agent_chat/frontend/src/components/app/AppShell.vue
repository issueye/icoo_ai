<script setup>
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import AppGlobalHeader from './AppGlobalHeader.vue'
import AppNavRail from './AppNavRail.vue'
import AppFooter from './AppFooter.vue'
import ConversationSidebar from '@/components/conversation/ConversationSidebar.vue'
import ChatWorkspace from '@/components/chat/ChatWorkspace.vue'
import SettingsSidebar from '@/components/settings/SettingsSidebar.vue'
import SettingsWorkspace from '@/components/settings/SettingsWorkspace.vue'
import { useApprovalsStore } from '@/stores/approvals'
import { useAppStore } from '@/stores/app'
import { useAuditStore } from '@/stores/audit'
import { useConversationsStore } from '@/stores/conversations'
import { useMessagesStore } from '@/stores/messages'
import { useRunsStore } from '@/stores/runs'
import { useSkillsStore } from '@/stores/skills'

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
const isSettingsRoute = () => route.name === 'settings'
const activeSettingsSection = ref('gateway')

onMounted(async () => {
  await app.refreshGatewayStatus()
  statusTimer = setInterval(() => {
    app.refreshGatewayStatus()
  }, 3000)
  await Promise.all([
    conversations.loadConversations(),
    runs.loadRuns(),
    approvals.loadApprovals(),
    skills.loadSkills(),
    audit.loadAuditEvents(),
  ])
  const routeSessionId = route.params.sessionId
  if (routeSessionId) conversations.setActiveSession(String(routeSessionId))
  if (conversations.activeSessionId) {
    await messages.loadMessages(conversations.activeSessionId)
    if (!routeSessionId) router.replace(`/chats/${conversations.activeSessionId}`)
  }
})

onBeforeUnmount(() => {
  if (statusTimer) {
    clearInterval(statusTimer)
    statusTimer = null
  }
})

watch(() => route.params.sessionId, async (sessionId) => {
  if (isSettingsRoute()) return
  if (!sessionId) return
  conversations.setActiveSession(String(sessionId))
  await messages.loadMessages(String(sessionId))
})

watch(() => route.name, (name) => {
  if (name === 'settings') app.setActiveNav('settings')
  else if (name === 'audit') app.setActiveNav('audit')
  else if (name === 'skills') app.setActiveNav('skills')
  else app.setActiveNav('chats')
}, { immediate: true })

watch(() => conversations.activeSessionId, async (sessionId) => {
  if (isSettingsRoute()) return
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
      <template v-else>
        <ConversationSidebar />
        <ChatWorkspace />
      </template>
    </div>
    <AppFooter />
  </main>
</template>
