<script setup>
import { GitBranch, MessageCircle } from 'lucide-vue-next'
import { formatShortTime } from '@/lib/utils'

defineProps({ conversation: { type: Object, required: true }, active: { type: Boolean, default: false } })
defineEmits(['select'])
</script>

<template>
  <button class="w-full rounded-2xl p-3 text-left transition" :class="active ? 'bg-blue-500 text-white shadow-panel' : 'bg-white text-slate-700 hover:bg-blue-50'" @click="$emit('select')">
    <div class="flex items-start gap-3">
      <div class="grid h-10 w-10 shrink-0 place-items-center rounded-2xl" :class="conversation.id.startsWith('subsess_') ? 'bg-violet-100 text-violet-600' : active ? 'bg-white/20 text-white' : 'bg-blue-100 text-blue-600'">
        <GitBranch v-if="conversation.id.startsWith('subsess_')" class="h-5 w-5" />
        <MessageCircle v-else class="h-5 w-5" />
      </div>
      <div class="min-w-0 flex-1">
        <div class="flex items-center justify-between gap-2">
          <p class="truncate text-sm font-semibold">{{ conversation.title }}</p>
          <span class="text-[11px] opacity-70">{{ formatShortTime(conversation.updatedAt) }}</span>
        </div>
        <p class="mt-1 truncate text-xs opacity-75">{{ conversation.subtitle }}</p>
        <div class="mt-2 flex items-center gap-2 text-[10px] font-medium">
          <span class="rounded-full px-2 py-0.5" :class="conversation.id.startsWith('subsess_') ? 'bg-violet-100 text-violet-700' : active ? 'bg-white/20 text-white' : 'bg-blue-50 text-blue-700'">{{ conversation.id.startsWith('subsess_') ? 'subsess_' : 'sess_' }}</span>
          <span class="opacity-70">{{ conversation.status }}</span>
        </div>
      </div>
    </div>
  </button>
</template>
