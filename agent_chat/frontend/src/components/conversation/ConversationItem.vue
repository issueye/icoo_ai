<script setup>
import { GitBranch, MessageCircle } from 'lucide-vue-next'
import { formatShortTime } from '@/lib/utils'

defineProps({ conversation: { type: Object, required: true }, active: { type: Boolean, default: false } })
defineEmits(['select', 'connect', 'disconnect', 'delete'])
</script>

<template>
  <article class="qq-conversation-item" :class="{ 'is-active': active }" @click="$emit('select')">
    <div class="flex items-start gap-3">
      <div class="qq-avatar" :class="{ 'is-subagent': conversation.id.startsWith('subsess_') }">
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
          <span class="qq-session-pill" :class="{ 'is-subagent': conversation.id.startsWith('subsess_') }">{{ conversation.id.startsWith('subsess_') ? 'subsess_' : 'sess_' }}</span>
          <span class="opacity-70">{{ conversation.status }}</span>
          <div class="ml-auto qq-conversation-actions">
            <button
              class="qq-secondary-action h-6 px-2 text-[10px] disabled:cursor-not-allowed disabled:opacity-55"
              type="button"
              :disabled="conversation.status === 'active'"
              @click.stop="$emit('connect')"
            >
              连接
            </button>
            <button
              class="qq-secondary-action h-6 px-2 text-[10px] disabled:cursor-not-allowed disabled:opacity-55"
              type="button"
              :disabled="conversation.status !== 'active'"
              @click.stop="$emit('disconnect')"
            >
              断开
            </button>
            <button
              class="qq-secondary-action qq-crud-danger h-6 px-2 text-[10px]"
              type="button"
              @click.stop="$emit('delete')"
            >
              删除
            </button>
          </div>
        </div>
      </div>
    </div>
  </article>
</template>
