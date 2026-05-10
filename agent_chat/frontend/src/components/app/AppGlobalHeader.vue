<script setup>
import { computed } from 'vue'
import { Bell, Maximize2, Minus, Search, X } from 'lucide-vue-next'
import { Window } from '@wailsio/runtime'
import { useAppStore } from '@/stores/app'
import { useApprovalsStore } from '@/stores/approvals'
import { useConversationsStore } from '@/stores/conversations'

const app = useAppStore()
const approvals = useApprovalsStore()
const conversations = useConversationsStore()

const navLabel = computed(() => {
  const labels = {
    chats: '消息',
    agents: 'Agent',
    skills: 'Skills',
    audit: '审计',
    settings: '设置',
  }
  return labels[app.activeNav] ?? '消息'
})

const title = computed(() => conversations.activeConversation?.title ?? 'Agent Chat')
const headerTitle = computed(() => {
  if (app.activeNav === 'audit') return '审计日志'
  if (app.activeNav === 'settings') return '系统设置'
  return title.value
})

function safeWindowAction(action) {
  action?.().catch(() => {})
}
</script>

<template>
  <header class="qq-global-header">
    <div class="qq-window-drag flex min-w-0 flex-1 items-center gap-3">
      <div class="qq-header-brand">
        <span class="qq-header-logo">AI</span>
        <span class="qq-header-product">icoo agent</span>
      </div>
      <div class="h-4 w-px bg-[color:var(--qq-border-strong)]" />
      <div class="min-w-0">
        <div class="flex items-center gap-2">
          <span class="qq-header-section">{{ navLabel }}</span>
          <span class="qq-header-title truncate">{{ headerTitle }}</span>
        </div>
      </div>
    </div>

    <div class="qq-window-no-drag flex items-center gap-2">
      <button class="qq-header-tool" aria-label="全局搜索">
        <Search class="h-4 w-4" />
      </button>
      <button class="qq-header-tool relative" aria-label="通知">
        <Bell class="h-4 w-4" />
        <span v-if="approvals.pendingCount" class="qq-dot" />
      </button>
      <button class="qq-window-control" aria-label="最小化" @click="safeWindowAction(Window.Minimise)">
        <Minus class="h-4 w-4" />
      </button>
      <button class="qq-window-control" aria-label="最大化或还原" @click="safeWindowAction(Window.ToggleMaximise)">
        <Maximize2 class="h-4 w-4" />
      </button>
      <button class="qq-window-control is-close" aria-label="关闭" @click="safeWindowAction(Window.Close)">
        <X class="h-4 w-4" />
      </button>
    </div>
  </header>
</template>
