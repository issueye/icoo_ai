<script setup>
import { computed, onMounted } from 'vue'
import { RefreshCcw, Search, ShieldCheck } from 'lucide-vue-next'
import ContextDropdown from '@/components/ui/ContextDropdown.vue'
import { useAuditStore } from '@/stores/audit'

const audit = useAuditStore()

const query = computed({
  get: () => audit.query,
  set: (value) => audit.setQuery(value),
})

const levelFilter = computed({
  get: () => audit.levelFilter,
  set: (value) => audit.setLevelFilter(value),
})

const typeFilter = computed({
  get: () => audit.typeFilter,
  set: (value) => audit.setTypeFilter(value),
})

const sessionFilter = computed({
  get: () => audit.sessionFilter,
  set: (value) => audit.setSessionFilter(value),
})

const filteredItems = computed(() => audit.filteredItems)
const selectedEvent = computed(() => audit.selectedEvent)
const summaryStats = computed(() => {
  const base = { total: audit.totalCount, debug: 0, info: 0, warn: 0, error: 0 }
  audit.items.forEach((item) => {
    if (item.level === 'debug') base.debug += 1
    else if (item.level === 'warn') base.warn += 1
    else if (item.level === 'error') base.error += 1
    else base.info += 1
  })
  return base
})

onMounted(async () => {
  if (!audit.items.length) {
    await audit.loadAuditEvents()
  }
  audit.markViewed()
})

function levelClass(level) {
  const normalized = String(level || '').trim().toLowerCase()
  if (normalized === 'error') return 'is-error'
  if (normalized === 'warn') return 'is-warn'
  if (normalized === 'debug') return 'is-debug'
  return 'is-info'
}

function levelLabel(level) {
  return String(level || 'info').toUpperCase()
}

function formatTime(value) {
  const date = new Date(value || '')
  if (Number.isNaN(date.getTime())) return '-'
  return date.toLocaleString('zh-CN', { hour12: false })
}

function selectEvent(eventID) {
  audit.setSelectedEvent(eventID)
}

async function refreshAuditEvents() {
  await audit.loadAuditEvents()
  audit.markViewed()
}

function clearFilters() {
  audit.clearFilters()
}
</script>

<template>
  <section class="qq-chat-workspace qq-audit-workspace">
    <header class="qq-chat-header qq-settings-header">
      <div class="min-w-0 flex-1">
        <h2 class="qq-chat-title">审计日志</h2>
        <p class="qq-sidebar-subtitle">系统行为与安全事件追踪</p>
      </div>
      <button class="qq-icon-button" :disabled="audit.loading" aria-label="刷新审计日志" @click="refreshAuditEvents">
        <RefreshCcw class="h-4 w-4" />
      </button>
    </header>

    <div class="qq-audit-layout">
      <div class="qq-audit-stats">
        <div class="qq-audit-stat-card">
          <div class="qq-audit-stat-label">总事件</div>
          <div class="qq-audit-stat-value">{{ summaryStats.total }}</div>
        </div>
        <div class="qq-audit-stat-card">
          <div class="qq-audit-stat-label">INFO</div>
          <div class="qq-audit-stat-value">{{ summaryStats.info }}</div>
        </div>
        <div class="qq-audit-stat-card">
          <div class="qq-audit-stat-label">WARN</div>
          <div class="qq-audit-stat-value">{{ summaryStats.warn }}</div>
        </div>
        <div class="qq-audit-stat-card">
          <div class="qq-audit-stat-label">ERROR</div>
          <div class="qq-audit-stat-value">{{ summaryStats.error }}</div>
        </div>
      </div>

      <div class="qq-audit-filters">
        <label class="qq-search-box qq-audit-search text-sm">
          <Search class="h-4 w-4" />
          <input v-model="query" class="w-full bg-transparent outline-none" placeholder="搜索摘要 / session / 类型 / ID" />
        </label>
        <ContextDropdown v-model="levelFilter" class="qq-audit-dropdown" label="等级" :options="audit.levelOptions" />
        <ContextDropdown v-model="typeFilter" class="qq-audit-dropdown" label="类型" :options="audit.typeOptions" />
        <ContextDropdown v-model="sessionFilter" class="qq-audit-dropdown" label="会话" :options="audit.sessionOptions" />
        <button class="qq-secondary-action h-8 px-3 text-sm" :disabled="audit.loading" @click="clearFilters">清空筛选</button>
      </div>

      <div class="qq-audit-grid">
        <section class="qq-audit-panel">
          <h3 class="qq-audit-panel-title">事件列表</h3>
          <div class="qq-audit-list scrollbar-thin-blue">
            <div v-if="audit.loading" class="qq-event-card text-sm">审计日志加载中...</div>
            <div v-else-if="audit.error" class="qq-event-card is-warning text-sm">{{ audit.error }}</div>
            <div v-else-if="!filteredItems.length" class="qq-event-card text-sm">当前筛选条件下没有审计事件</div>
            <template v-else>
              <button
                v-for="item in filteredItems"
                :key="item.id"
                class="qq-audit-row"
                :class="{ 'is-active': selectedEvent?.id === item.id }"
                @click="selectEvent(item.id)"
              >
                <div class="qq-audit-row-head">
                  <span class="qq-audit-level" :class="levelClass(item.level)">{{ levelLabel(item.level) }}</span>
                  <span class="qq-audit-time">{{ formatTime(item.createdAt) }}</span>
                </div>
                <p class="qq-audit-summary">{{ item.summary }}</p>
                <div class="qq-audit-meta-row">
                  <span class="qq-audit-meta-chip">{{ item.type }}</span>
                  <span class="qq-audit-meta-chip">{{ item.sessionId || 'global' }}</span>
                </div>
              </button>
            </template>
          </div>
        </section>

        <section class="qq-audit-panel">
          <h3 class="qq-audit-panel-title">事件详情</h3>
          <div v-if="selectedEvent" class="qq-audit-detail">
            <div class="qq-audit-detail-row">
              <span class="qq-audit-detail-label">事件 ID</span>
              <span class="qq-audit-detail-value">{{ selectedEvent.id }}</span>
            </div>
            <div class="qq-audit-detail-row">
              <span class="qq-audit-detail-label">会话 ID</span>
              <span class="qq-audit-detail-value">{{ selectedEvent.sessionId || 'global' }}</span>
            </div>
            <div class="qq-audit-detail-row">
              <span class="qq-audit-detail-label">类型</span>
              <span class="qq-audit-detail-value">{{ selectedEvent.type }}</span>
            </div>
            <div class="qq-audit-detail-row">
              <span class="qq-audit-detail-label">等级</span>
              <span class="qq-audit-detail-value">
                <span class="qq-audit-level" :class="levelClass(selectedEvent.level)">{{ levelLabel(selectedEvent.level) }}</span>
              </span>
            </div>
            <div class="qq-audit-detail-row">
              <span class="qq-audit-detail-label">时间</span>
              <span class="qq-audit-detail-value">{{ formatTime(selectedEvent.createdAt) }}</span>
            </div>
            <div class="qq-audit-detail-row is-block">
              <span class="qq-audit-detail-label">摘要</span>
              <p class="qq-audit-detail-summary">{{ selectedEvent.summary }}</p>
            </div>
          </div>
          <div v-else class="qq-event-card text-sm">
            <ShieldCheck class="mr-1 inline h-4 w-4" />
            暂无事件详情
          </div>
        </section>
      </div>
    </div>
  </section>
</template>
