<script setup>
import { computed, ref } from 'vue'
import { Send, Square } from 'lucide-vue-next'
import { useConversationsStore } from '@/stores/conversations'
import { useMessagesStore } from '@/stores/messages'
import { useRunsStore } from '@/stores/runs'

const draft = ref('')
const conversations = useConversationsStore()
const messages = useMessagesStore()
const runs = useRunsStore()
const activeSessionId = computed(() => conversations.activeSessionId)
const sending = computed(() => Boolean(messages.sendingBySessionId[activeSessionId.value]))

async function sendPrompt() {
  if (!activeSessionId.value || sending.value || !draft.value.trim()) return
  const prompt = draft.value
  draft.value = ''
  await messages.sendPrompt(activeSessionId.value, prompt)
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
  <footer class="border-t border-white/70 bg-[#f8fbff] p-5">
    <div class="mx-auto max-w-4xl rounded-3xl bg-white p-3 shadow-panel">
      <textarea v-model="draft" class="h-20 w-full resize-none rounded-2xl bg-slate-50 p-3 text-sm outline-none focus:ring-2 focus:ring-blue-200" placeholder="输入消息，或使用 /skill 启动 subagent..." @keydown="handleKeydown" />
      <div class="mt-3 flex items-center justify-between gap-3 text-xs text-slate-500">
        <span>Enter 发送 · Shift+Enter 换行 · 工具大输出仅保存摘要</span>
        <div class="flex items-center gap-2">
          <button class="inline-flex items-center gap-1 rounded-2xl bg-white px-3 py-2 text-sm font-medium text-slate-600 ring-1 ring-slate-200 hover:bg-slate-50" aria-label="停止运行" @click="cancelRun">
            <Square class="h-4 w-4" />
            停止
          </button>
          <button class="inline-flex items-center gap-1 rounded-2xl bg-blue-500 px-4 py-2 text-sm font-medium text-white disabled:cursor-not-allowed disabled:opacity-50" :disabled="sending || !draft.trim()" aria-label="发送消息" @click="sendPrompt">
            <Send class="h-4 w-4" />
            {{ sending ? '发送中' : '发送' }}
          </button>
        </div>
      </div>
    </div>
  </footer>
</template>
