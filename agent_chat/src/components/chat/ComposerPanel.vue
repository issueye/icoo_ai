<template>
  <footer class="border-t border-white/70 bg-[#f8fbff] p-5">
    <div class="mx-auto max-w-4xl rounded-3xl bg-white p-3 shadow-panel">
      <textarea class="h-20 w-full resize-none rounded-2xl bg-slate-50 p-3 text-sm outline-none focus:ring-2 focus:ring-blue-200" placeholder="输入消息，或使用 /skill 启动 subagent..." />
      <div class="mt-3 flex items-center justify-between text-xs text-slate-500">
        <span>Enter 发送 · Shift+Enter 换行 · 工具大输出仅保存摘要</span>
        <button class="rounded-2xl bg-blue-500 px-4 py-2 text-sm font-medium text-white" aria-label="发送消息">发送</button>
      </div>
    </div>
  </footer>
</template>
