<script setup>
import { onMounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import AppNavRail from './AppNavRail.vue'
import ConversationSidebar from '@/components/conversation/ConversationSidebar.vue'
import ChatWorkspace from '@/components/chat/ChatWorkspace.vue'
import { useApprovalsStore } from '@/stores/approvals'
import { useAuditStore } from '@/stores/audit'
import { useConversationsStore } from '@/stores/conversations'
import { useMessagesStore } from '@/stores/messages'
import { useRunsStore } from '@/stores/runs'
import { useSkillsStore } from '@/stores/skills'

const route = useRoute()
const router = useRouter()
const conversations = useConversationsStore()
const messages = useMessagesStore()
const runs = useRunsStore()
const approvals = useApprovalsStore()
const skills = useSkillsStore()
const audit = useAuditStore()

onMounted(async () => {
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

watch(() => route.params.sessionId, async (sessionId) => {
  if (!sessionId) return
  conversations.setActiveSession(String(sessionId))
  await messages.loadMessages(String(sessionId))
})

watch(() => conversations.activeSessionId, async (sessionId) => {
  if (!sessionId) return
  if (route.params.sessionId !== sessionId) router.push(`/chats/${sessionId}`)
  await messages.loadMessages(sessionId)
})
</script>

<template>
  <main class="flex h-screen overflow-hidden bg-[#eaf3fb] text-slate-900">
    <AppNavRail />
    <ConversationSidebar />
    <ChatWorkspace />
  </main>
</template>
