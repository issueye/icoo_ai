<script setup>
import { computed, onMounted, ref } from 'vue'
import { Pencil, Plus, Power, PowerOff, Save, Trash2 } from 'lucide-vue-next'
import { useAppStore } from '@/stores/app'

const app = useAppStore()
const tasks = ref([])
const editingIndex = ref(-1)
const draft = ref(createEmptyTask())

function createEmptyTask() {
  return {
    id: '',
    name: '',
    spec: '*/5 * * * *',
    command: '',
    argsText: '',
    enabled: true,
  }
}

function toDraft(task = {}) {
  return {
    id: task?.id || '',
    name: task?.name || '',
    spec: task?.spec || '*/5 * * * *',
    command: task?.command || '',
    argsText: Array.isArray(task?.args) ? task.args.join(' ') : '',
    enabled: Boolean(task?.enabled),
  }
}

function normalizeDraftTask(value, fallbackIndex = 1) {
  const id = String(value?.id || '').trim() || `task_${fallbackIndex}`
  const name = String(value?.name || '').trim() || id
  const spec = String(value?.spec || '').trim() || '*/5 * * * *'
  const command = String(value?.command || '').trim()
  const args = String(value?.argsText || '')
    .trim()
    .split(/\s+/)
    .map((item) => item.trim())
    .filter(Boolean)
  return {
    id,
    name,
    spec,
    command,
    args,
    enabled: Boolean(value?.enabled),
  }
}

const disabled = computed(() => app.settingsSaving)
const isEditing = computed(() => editingIndex.value >= 0)

onMounted(async () => {
  await app.loadAppSettings()
  tasks.value = Array.isArray(app.scheduleTasks) ? app.scheduleTasks.map((item) => ({ ...item })) : []
})

function startCreate() {
  editingIndex.value = -1
  draft.value = createEmptyTask()
}

function startEdit(index) {
  editingIndex.value = index
  draft.value = toDraft(tasks.value[index])
}

function removeTask(index) {
  tasks.value.splice(index, 1)
}

function toggleTask(index) {
  const current = tasks.value[index]
  if (!current) return
  current.enabled = !current.enabled
}

function applyDraft() {
  const normalized = normalizeDraftTask(draft.value, tasks.value.length + 1)
  if (!normalized.command) {
    app.pushToast({ type: 'error', message: '请填写 command' })
    return
  }
  if (isEditing.value) {
    tasks.value[editingIndex.value] = normalized
  } else {
    tasks.value.push(normalized)
  }
  startCreate()
}

async function saveScheduleSettings() {
  try {
    await app.saveAppSettings({ scheduleTasks: tasks.value })
    tasks.value = Array.isArray(app.scheduleTasks) ? app.scheduleTasks.map((item) => ({ ...item })) : []
    app.pushToast({ type: 'success', message: '定时任务配置保存成功' })
  } catch {
    app.pushToast({ type: 'error', message: app.settingsError || '定时任务配置保存失败' })
  }
}
</script>

<template>
  <section class="qq-chat-workspace">
    <header class="qq-chat-header qq-settings-header">
      <div class="min-w-0 flex-1">
        <h2 class="qq-chat-title">定时任务管理</h2>
        <p class="qq-sidebar-subtitle">配置任务（id / name / spec / command / args / enabled）</p>
      </div>
    </header>
    <div class="qq-settings-body">
      <div class="qq-settings-card qq-schedule-panel">
        <div class="qq-schedule-form">
          <label class="qq-settings-label" for="taskId">ID</label>
          <input id="taskId" v-model="draft.id" type="text" class="qq-settings-input" placeholder="task_id" />
          <label class="qq-settings-label" for="taskName">名称</label>
          <input id="taskName" v-model="draft.name" type="text" class="qq-settings-input" placeholder="定时任务" />
          <label class="qq-settings-label" for="taskSpec">Cron 表达式</label>
          <input id="taskSpec" v-model="draft.spec" type="text" class="qq-settings-input" placeholder="*/5 * * * *" />
          <label class="qq-settings-label" for="taskCommand">Command</label>
          <input id="taskCommand" v-model="draft.command" type="text" class="qq-settings-input" placeholder="node" />
          <label class="qq-settings-label" for="taskArgs">Args（空格分隔）</label>
          <input id="taskArgs" v-model="draft.argsText" type="text" class="qq-settings-input" placeholder="scripts/job.js --env prod" />
          <div class="qq-schedule-form-actions">
            <button class="qq-secondary-action h-9 px-4" :disabled="disabled" @click="startCreate">
              <Plus class="h-4 w-4" />
              <span>新增</span>
            </button>
            <button class="qq-primary-action h-9 px-4" :disabled="disabled" @click="applyDraft">
              <Pencil class="h-4 w-4" />
              <span>{{ isEditing ? '更新条目' : '加入列表' }}</span>
            </button>
          </div>
        </div>

        <div class="qq-schedule-list">
          <div v-if="tasks.length === 0" class="qq-settings-helper">暂无定时任务，请先新增。</div>
          <div v-for="(task, index) in tasks" :key="`${task.id}_${index}`" class="qq-schedule-item">
            <div class="qq-schedule-item-main">
              <div class="qq-schedule-item-title">{{ task.name }}（{{ task.id }}）</div>
              <div class="qq-schedule-item-subtitle">{{ task.spec }} · {{ task.command }} {{ (task.args || []).join(' ') }}</div>
            </div>
            <div class="qq-schedule-item-actions">
              <button class="qq-icon-button" :disabled="disabled" aria-label="编辑条目" @click="startEdit(index)">
                <Pencil class="h-4 w-4" />
              </button>
              <button class="qq-icon-button" :disabled="disabled" aria-label="切换启用状态" @click="toggleTask(index)">
                <Power v-if="task.enabled" class="h-4 w-4" />
                <PowerOff v-else class="h-4 w-4" />
              </button>
              <button class="qq-icon-button qq-schedule-danger" :disabled="disabled" aria-label="删除条目" @click="removeTask(index)">
                <Trash2 class="h-4 w-4" />
              </button>
            </div>
          </div>
        </div>

        <div class="qq-settings-actions qq-schedule-save-row">
          <button class="qq-primary-action h-9 px-4" :disabled="disabled" @click="saveScheduleSettings">
            <Save class="h-4 w-4" />
            <span>{{ app.settingsSaving ? '保存中' : '保存定时任务配置' }}</span>
          </button>
          <span v-if="app.settingsError" class="qq-settings-error">{{ app.settingsError }}</span>
        </div>
      </div>
    </div>
  </section>
</template>
