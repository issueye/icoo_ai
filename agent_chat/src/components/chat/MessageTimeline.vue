<script setup>
import ApprovalCard from './ApprovalCard.vue'
import MessageBubble from './MessageBubble.vue'
import SubagentRunCard from './SubagentRunCard.vue'
import ToolCallCard from './ToolCallCard.vue'
import { useMessagesStore } from '@/stores/messages'

const messages = useMessagesStore()
</script>

<template>
  <div class="scrollbar-thin-blue flex-1 overflow-y-auto px-8 py-6">
    <div class="mx-auto flex max-w-4xl flex-col gap-4">
      <template v-for="item in messages.activeItems" :key="item.id">
        <MessageBubble v-if="item.kind === 'message'" :message="item" />
        <ToolCallCard v-else-if="item.kind === 'tool_call'" :event="item" />
        <ApprovalCard v-else-if="item.kind === 'approval'" :event="item" />
        <SubagentRunCard v-else-if="item.kind === 'subagent_run'" :event="item" />
      </template>
    </div>
  </div>
</template>
