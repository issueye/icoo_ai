<script setup>
import { useRouter } from 'vue-router'
import ApprovalCard from './ApprovalCard.vue'
import MessageBubble from './MessageBubble.vue'
import SubagentRunCard from './SubagentRunCard.vue'
import ToolCallCard from './ToolCallCard.vue'
import { useConversationsStore } from '@/stores/conversations'
import { useMessagesStore } from '@/stores/messages'

const messages = useMessagesStore()
const conversations = useConversationsStore()
const router = useRouter()

function openSession(sessionId) {
  conversations.setActiveSession(sessionId)
  router.push(`/chats/${sessionId}`)
}
</script>

<template>
  <div class="qq-message-timeline scrollbar-thin-blue">
    <div class="qq-message-stack">
      <template v-for="item in messages.activeItems" :key="item.id">
        <MessageBubble v-if="item.kind === 'message'" :message="item" />
        <ToolCallCard v-else-if="item.kind === 'tool_call'" :event="item" />
        <ApprovalCard v-else-if="item.kind === 'approval'" :event="item" />
        <SubagentRunCard v-else-if="item.kind === 'subagent_run'" :event="item" @open-session="openSession" />
      </template>
    </div>
  </div>
</template>
