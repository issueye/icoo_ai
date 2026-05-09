<script setup>
import { CheckCircle2, Info, TriangleAlert, X } from 'lucide-vue-next'
import { useAppStore } from '@/stores/app'

const app = useAppStore()

function resolveIcon(type) {
  if (type === 'success') return CheckCircle2
  if (type === 'error') return TriangleAlert
  return Info
}
</script>

<template>
  <aside class="qq-toast-stack" aria-live="polite">
    <article v-for="toast in app.toasts" :key="toast.id" class="qq-toast" :class="`is-${toast.type}`">
      <component :is="resolveIcon(toast.type)" class="qq-toast-icon" />
      <p class="qq-toast-message">{{ toast.message }}</p>
      <button class="qq-toast-close" type="button" aria-label="关闭提醒" @click="app.removeToast(toast.id)">
        <X class="h-3.5 w-3.5" />
      </button>
    </article>
  </aside>
</template>
