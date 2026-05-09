<script setup>
import { computed, ref } from 'vue'
import { Boxes, BrainCircuit, FolderGit2, Send, Square } from 'lucide-vue-next'
import { useConversationsStore } from '@/stores/conversations'
import { useMessagesStore } from '@/stores/messages'
import { useRunsStore } from '@/stores/runs'
import ContextDropdown from '@/components/ui/ContextDropdown.vue'

const draft = ref('')
const conversations = useConversationsStore()
const messages = useMessagesStore()
const runs = useRunsStore()
const activeSessionId = computed(() => conversations.activeSessionId)
const sending = computed(() => Boolean(messages.sendingBySessionId[activeSessionId.value]))
const activeContextText = computed(() => `${conversations.activeWorkspace.label} · ${conversations.activeMode.label} · ${conversations.activeModel.label}`)

async function sendPrompt() {
  if (!activeSessionId.value || sending.value || !draft.value.trim()) return
  const prompt = draft.value
  draft.value = ''
  await messages.sendPrompt(activeSessionId.value, prompt, {
    workspaceId: conversations.activeWorkspace.id,
    cwd: conversations.activeWorkspace.path,
    mode: conversations.activeMode.id,
    model: conversations.activeModel.id,
  })
  await conversations.loadConversations()
}

async function cancelRun() {
  if (!activeSessionId.value) return
  await runs.cancel(activeSessionId.value)
  await conversations.loadConversations()
}

function handleKeydown(event) {
  if (event.key === 'Enter' && !event.shiftKey) {
    event.preventDefault()
    sendPrompt()
  }
}
</script>

<template>
  <footer class="qq-composer">
    <div class="qq-composer-inner">
      <textarea v-model="draft" class="qq-textarea" placeholder="输入消息，或使用 /skill 启动 subagent..."
        @keydown="handleKeydown" />
      <div class="mt-1 flex items-center justify-between gap-3 text-xs text-[color:var(--qq-text-muted)]">
        <div class="qq-context-bar">
          <ContextDropdown label="工作区" :icon="FolderGit2"
            :model-value="conversations.activeConversation?.workspaceId ?? conversations.activeWorkspace.id"
            :options="conversations.workspaceOptions"
            @update:model-value="conversations.updateActiveContext({ workspaceId: $event })" />
          <ContextDropdown label="模式" :icon="Boxes"
            :model-value="conversations.activeConversation?.mode ?? conversations.activeMode.id"
            :options="conversations.modeOptions"
            @update:model-value="conversations.updateActiveContext({ mode: $event })" />
          <ContextDropdown label="模型" :icon="BrainCircuit"
            :model-value="conversations.activeConversation?.model ?? conversations.activeModel.id"
            :options="conversations.modelOptions"
            @update:model-value="conversations.updateActiveContext({ model: $event })" />
        </div>

        <div class="flex items-center gap-2">
          <button class="qq-secondary-action h-8 px-3 text-sm font-medium" aria-label="停止运行" @click="cancelRun">
            <Square class="h-4 w-4" />
            停止
          </button>
          <button class="qq-primary-action h-8 px-4 text-sm" :disabled="sending || !draft.trim()" aria-label="发送消息"
            @click="sendPrompt">
            <Send class="h-4 w-4" />
            {{ sending ? '发送中' : '发送' }}
          </button>
        </div>
      </div>
    </div>
  </footer>
</template>
