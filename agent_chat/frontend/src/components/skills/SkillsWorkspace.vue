<script setup>
import { computed, onMounted, ref } from 'vue'
import CrudModuleWorkspace from '@/components/management/CrudModuleWorkspace.vue'
import { useSkillsStore } from '@/stores/skills'

const skills = useSkillsStore()
const items = ref([])

const tableColumns = [
  { key: 'id', label: 'ID' },
  { key: 'name', label: '名称' },
  { key: 'description', label: '描述' },
]

const detailFields = [
  { key: 'id', label: 'ID' },
  { key: 'name', label: '名称' },
  { key: 'description', label: '描述' },
]

const subtitle = computed(() => {
  if (!skills.lastLoadedAt) return '查询、分页查看当前技能清单'
  const date = new Date(skills.lastLoadedAt)
  if (Number.isNaN(date.getTime())) return '查询、分页查看当前技能清单'
  return `查询、分页查看当前技能清单（更新时间 ${date.toLocaleString('zh-CN', { hour12: false })}）`
})

onMounted(async () => {
  if (!skills.items.length && !skills.loading) await skills.loadSkills()
  items.value = skills.items.map((item) => ({ ...item }))
})

async function refreshSkills() {
  await skills.loadSkills()
  items.value = skills.items.map((item) => ({ ...item }))
}
</script>

<template>
  <CrudModuleWorkspace
    v-model:items="items"
    title="技能管理"
    :subtitle="subtitle"
    empty-text="当前筛选条件下没有技能"
    query-placeholder="按技能 ID / 名称 / 描述搜索"
    :table-columns="tableColumns"
    :detail-fields="detailFields"
    :loading="skills.loading"
    :error-text="skills.error"
    :show-refresh="true"
    action-mode="view"
    detail-title="技能详情"
    :allow-create="false"
    :allow-edit="false"
    :allow-delete="false"
    :allow-toggle="false"
    :allow-save="false"
    :readonly="true"
    @refresh="refreshSkills"
  />
</template>
