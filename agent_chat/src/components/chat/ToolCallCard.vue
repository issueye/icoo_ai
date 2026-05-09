<script setup>
import { ChevronDown, TerminalSquare } from 'lucide-vue-next'
defineProps({ event: { type: Object, required: true } })
</script>

<template>
  <article class="rounded-2xl border border-blue-100 bg-white/85 p-4 text-sm shadow-sm">
    <div class="flex items-center justify-between">
      <div class="flex items-center gap-2 font-semibold text-slate-700"><TerminalSquare class="h-4 w-4 text-blue-500" /> tool: {{ event.toolName }}</div>
      <span class="rounded-full bg-emerald-100 px-2 py-0.5 text-xs text-emerald-700">{{ event.status }} · {{ event.durationMs }}ms</span>
    </div>
    <p class="mt-2 text-slate-600">{{ event.summary }}</p>
    <dl class="mt-3 grid grid-cols-3 gap-2 text-xs text-slate-500">
      <div class="rounded-xl bg-slate-50 p-2"><dt>输出大小</dt><dd>{{ event.safeMeta.outputBytes }} bytes</dd></div>
      <div class="rounded-xl bg-slate-50 p-2"><dt>摘要 Hash</dt><dd>{{ event.safeMeta.outputHash }}</dd></div>
      <div class="rounded-xl bg-slate-50 p-2"><dt>完整输出落盘</dt><dd>{{ event.safeMeta.persistedOutput ? '是' : '否' }}</dd></div>
    </dl>
    <button class="mt-3 flex items-center gap-1 text-xs text-blue-600" aria-label="展开工具安全摘要"><ChevronDown class="h-3.5 w-3.5" />仅展示安全摘要</button>
  </article>
</template>
