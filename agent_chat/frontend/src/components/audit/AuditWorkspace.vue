<script setup>
import { onMounted, ref } from 'vue'
import CrudModuleWorkspace from '@/components/management/CrudModuleWorkspace.vue'
import { useAuditStore } from '@/stores/audit'

const audit = useAuditStore()
const items = ref([])

function formatTime(value) {
  const date = new Date(value || '')
  if (Number.isNaN(date.getTime())) return '-'
  return date.toLocaleString('zh-CN', { hour12: false })
}

function levelLabel(level) {
  return String(level || 'info').toUpperCase()
}

const tableColumns = [
  { key: 'createdAt', label: '时间', formatter: (item) => formatTime(item.createdAt) },
  { key: 'level', label: '等级', formatter: (item) => levelLabel(item.level) },
  { key: 'type', label: '类型' },
  { key: 'sessionId', label: '会话', formatter: (item) => item.sessionId || 'global' },
  { key: 'summary', label: '摘要' },
]

const detailFields = [
  { key: 'id', label: '事件 ID' },
  { key: 'createdAt', label: '时间', formatter: (item) => formatTime(item.createdAt) },
  { key: 'level', label: '等级', formatter: (item) => levelLabel(item.level) },
  { key: 'type', label: '类型' },
  { key: 'sessionId', label: '会话', formatter: (item) => item.sessionId || 'global' },
  { key: 'summary', label: '摘要' },
]

onMounted(async () => {
  if (!audit.items.length) await audit.loadAuditEvents()
  audit.markViewed()
  items.value = audit.items.map((item) => ({ ...item }))
})

async function refreshAudit() {
  await audit.loadAuditEvents()
  audit.markViewed()
  items.value = audit.items.map((item) => ({ ...item }))
}
</script>

<template>
  <CrudModuleWorkspace
    v-model:items="items"
    title="审计日志"
    subtitle="查询、分页查看系统审计事件"
    empty-text="当前筛选条件下没有审计事件"
    query-placeholder="按事件 ID / 会话 / 类型 / 摘要搜索"
    :table-columns="tableColumns"
    :detail-fields="detailFields"
    :loading="audit.loading"
    :error-text="audit.error || ''"
    :show-refresh="true"
    action-mode="view"
    detail-title="审计详情"
    :allow-create="false"
    :allow-edit="false"
    :allow-delete="false"
    :allow-toggle="false"
    :allow-save="false"
    :readonly="true"
    @refresh="refreshAudit"
  />
</template>
