<script setup>
import { ChevronDown, TerminalSquare } from 'lucide-vue-next'
defineProps({ event: { type: Object, required: true } })
</script>

<template>
  <article class="qq-event-card">
    <div class="flex items-center justify-between">
      <div class="flex items-center gap-2 font-semibold text-[color:var(--qq-text)]"><TerminalSquare class="h-4 w-4 text-[color:var(--qq-active)]" /> tool: {{ event.toolName }}</div>
      <span class="qq-status-pill">{{ event.status }} · {{ event.durationMs }}ms</span>
    </div>
    <p class="mt-2 text-[color:var(--qq-text-soft)]">{{ event.summary }}</p>
    <dl class="qq-meta-grid mt-3 text-xs text-[color:var(--qq-text-muted)]">
      <div class="qq-meta-cell"><dt>输出大小</dt><dd>{{ event.safeMeta.outputBytes }} bytes</dd></div>
      <div class="qq-meta-cell"><dt>摘要 Hash</dt><dd>{{ event.safeMeta.outputHash }}</dd></div>
      <div class="qq-meta-cell"><dt>完整输出落盘</dt><dd>{{ event.safeMeta.persistedOutput ? '是' : '否' }}</dd></div>
    </dl>
    <button class="mt-3 flex items-center gap-1 text-xs text-[color:var(--qq-active)]" aria-label="展开工具安全摘要"><ChevronDown class="h-3.5 w-3.5" />仅展示安全摘要</button>
  </article>
</template>
