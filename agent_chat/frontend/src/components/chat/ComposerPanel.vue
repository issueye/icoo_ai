<script setup>
import { computed, ref } from 'vue'
import { BookOpen, BrainCircuit, ChevronRight, Image, Paperclip, Send, Sparkles, Square } from 'lucide-vue-next'
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
const activeModeId = computed(() => conversations.activeConversation?.mode ?? conversations.activeMode?.id ?? '')
const activeWorkspaceId = computed(() => conversations.activeConversation?.workspaceId ?? conversations.activeWorkspace.id)
const activeModelId = computed(() => conversations.activeConversation?.model ?? conversations.activeModel?.id ?? '')

async function sendPrompt() {
  if (!activeSessionId.value || sending.value || !draft.value.trim()) return
  const prompt = draft.value
  draft.value = ''
  await messages.sendPrompt(activeSessionId.value, prompt, {
    workspaceId: conversations.activeWorkspace.id,
    cwd: conversations.activeWorkspace.path,
    mode: conversations.activeMode?.id || '',
    model: conversations.activeModel?.id || '',
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
    <div class="qq-composer-dialog">
      <div class="qq-composer-inner qq-composer-inner-elevated">
        <textarea
          v-model="draft"
          class="qq-textarea qq-textarea-flat"
          placeholder="Enter发送, Shift+Enter换行"
          @keydown="handleKeydown"
        />
        <div class="qq-composer-bottom">
          <div class="qq-composer-chip-row">
            <ContextDropdown
              class="qq-context-dropdown-chip"
              label="思考"
              :icon="Sparkles"
              :model-value="activeModeId"
              :options="conversations.modeOptions"
              @update:model-value="conversations.updateActiveContext({ mode: $event })"
            />
            <ContextDropdown
              class="qq-context-dropdown-chip"
              label="知识库"
              :icon="BookOpen"
              :model-value="activeWorkspaceId"
              :options="conversations.workspaceOptions"
              @update:model-value="conversations.updateActiveContext({ workspaceId: $event })"
            />
          </div>

          <div class="qq-composer-actions">
            <ContextDropdown
              class="qq-context-dropdown-model"
              label=""
              :icon="BrainCircuit"
              :model-value="activeModelId"
              :options="conversations.modelOptions"
              @update:model-value="conversations.updateActiveContext({ model: $event })"
            />
            <button class="qq-composer-tool" type="button" aria-label="上传附件">
              <Paperclip class="h-4 w-4" />
            </button>
            <button class="qq-composer-tool" type="button" aria-label="插入图片">
              <Image class="h-4 w-4" />
            </button>
            <button
              class="qq-composer-send"
              :disabled="(!draft.trim() && !sending) || !activeSessionId"
              aria-label="发送消息"
              @click="sending ? cancelRun() : sendPrompt()"
            >
              <Square v-if="sending" class="h-3.5 w-3.5" />
              <Send v-else class="h-4 w-4" />
            </button>
          </div>
        </div>
      </div>
    </div>
  </footer>
</template>
