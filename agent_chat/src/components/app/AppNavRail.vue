<script setup>
import { Bot, CheckSquare, MessageCircle, Settings, ShieldCheck, Sparkles } from 'lucide-vue-next'
import { useAppStore } from '@/stores/app'

const app = useAppStore()
const navItems = [
  { key: 'chats', label: '会话', icon: MessageCircle, badge: true },
  { key: 'agents', label: 'Agent', icon: Bot },
  { key: 'skills', label: 'Skills', icon: Sparkles },
  { key: 'audit', label: '审计', icon: ShieldCheck, badge: true },
]
</script>

<template>
  <aside class="flex w-[64px] flex-col items-center border-r border-white/60 bg-[#d8eafa] py-4">
    <div class="mb-6 grid h-10 w-10 place-items-center rounded-2xl bg-blue-500 text-sm font-bold text-white shadow-panel">AI</div>
    <nav class="flex flex-1 flex-col gap-2" aria-label="主导航">
      <button v-for="item in navItems" :key="item.key" class="relative grid h-11 w-11 place-items-center rounded-2xl transition" :class="app.activeNav === item.key ? 'bg-white text-blue-600 shadow-sm' : 'text-slate-500 hover:bg-white/70'" :aria-label="item.label" @click="app.setActiveNav(item.key)">
        <component :is="item.icon" class="h-5 w-5" />
        <span v-if="item.badge" class="absolute right-2 top-2 h-2 w-2 rounded-full bg-rose-500" />
      </button>
    </nav>
    <button class="grid h-11 w-11 place-items-center rounded-2xl text-slate-500 hover:bg-white/70" aria-label="待办审批">
      <CheckSquare class="h-5 w-5" />
    </button>
    <button class="mt-2 grid h-11 w-11 place-items-center rounded-2xl text-slate-500 hover:bg-white/70" aria-label="设置">
      <Settings class="h-5 w-5" />
    </button>
  </aside>
</template>
