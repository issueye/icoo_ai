<script setup>
import { Bot, CheckSquare, MessageCircle, Settings, ShieldCheck, Sparkles, Webhook } from 'lucide-vue-next'
import { useRouter } from 'vue-router'
import { useAppStore } from '@/stores/app'
import { useApprovalsStore } from '@/stores/approvals'
import { useAuditStore } from '@/stores/audit'
import { useConversationsStore } from '@/stores/conversations'

const app = useAppStore()
const approvals = useApprovalsStore()
const audit = useAuditStore()
const conversations = useConversationsStore()
const router = useRouter()
const navItems = [
  { key: 'chats', label: '会话', icon: MessageCircle },
  { key: 'channels', label: '渠道', icon: Webhook },
  { key: 'agents', label: 'Agent', icon: Bot },
  { key: 'skills', label: 'Skills', icon: Sparkles },
  { key: 'audit', label: '审计', icon: ShieldCheck, badge: true },
]

function navigate(item) {
  app.setActiveNav(item.key)
  if (item.key === 'chats') router.push(conversations.activeSessionId ? `/chats/${conversations.activeSessionId}` : '/chats')
  else if (item.key === 'channels') router.push('/channels')
  else if (item.key === 'skills') router.push('/skills')
  else if (item.key === 'audit') router.push('/audit')
  else if (item.key === 'settings') router.push('/settings')
}
</script>

<template>
  <aside class="qq-nav-rail">
    <div class="qq-brand-mark">AI</div>
    <nav class="flex flex-1 flex-col gap-2" aria-label="主导航">
      <button v-for="item in navItems" :key="item.key" class="qq-nav-button" :class="{ 'is-active': app.activeNav === item.key }" :aria-label="item.label" @click="navigate(item)">
        <component :is="item.icon" class="h-5 w-5" />
        <span v-if="item.badge && item.key === 'audit' && audit.unreadCount > 0" class="qq-dot" />
      </button>
    </nav>
    <button class="qq-nav-button" aria-label="待办审批">
      <CheckSquare class="h-5 w-5" />
      <span v-if="approvals.pendingCount" class="qq-dot" />
    </button>
    <button class="qq-nav-button mt-2" :class="{ 'is-active': app.activeNav === 'settings' }" aria-label="设置" @click="navigate({ key: 'settings' })">
      <Settings class="h-5 w-5" />
    </button>
  </aside>
</template>
