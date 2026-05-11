<script setup>
import { computed, ref, watch } from 'vue'
import { Pencil, Plus, Power, PowerOff, Save, Search, Trash2, X } from 'lucide-vue-next'

const props = defineProps({
  title: { type: String, required: true },
  subtitle: { type: String, default: '' },
  items: { type: Array, default: () => [] },
  saveLabel: { type: String, default: '保存配置' },
  emptyText: { type: String, default: '暂无数据，请先新增。' },
  queryPlaceholder: { type: String, default: '按 ID / 名称 / 命令搜索' },
  pageSize: { type: Number, default: 10 },
  extraFields: { type: Array, default: () => [] },
  formatSubtitle: { type: Function, default: (item) => `${item.command || ''} ${(item.args || []).join(' ')}`.trim() },
  validateItem: { type: Function, default: () => '' },
})

const emit = defineEmits(['update:items', 'save', 'error'])

const draft = ref(createEmptyItem())
const editingIndex = ref(-1)
const localItems = ref(copyItems(props.items))
const query = ref('')
const currentPage = ref(1)
const modalOpen = ref(false)

watch(() => props.items, (next) => {
  localItems.value = copyItems(next)
}, { deep: true })

function copyItems(items) {
  return Array.isArray(items) ? items.map((item) => ({ ...item, args: Array.isArray(item.args) ? [...item.args] : [] })) : []
}

function createEmptyItem() {
  const base = {
    id: '',
    name: '',
    command: '',
    argsText: '',
    enabled: true,
  }
  for (const field of props.extraFields) {
    base[field.key] = field.defaultValue ?? ''
  }
  return base
}

function toDraft(item = {}) {
  const out = {
    id: item.id || '',
    name: item.name || '',
    command: item.command || '',
    argsText: Array.isArray(item.args) ? item.args.join(' ') : '',
    enabled: Boolean(item.enabled),
  }
  for (const field of props.extraFields) {
    out[field.key] = item[field.key] ?? field.defaultValue ?? ''
  }
  return out
}

function normalizeDraft(input, sequence = 1) {
  const id = String(input?.id || '').trim() || `item_${sequence}`
  const name = String(input?.name || '').trim() || id
  const command = String(input?.command || '').trim()
  const args = String(input?.argsText || '')
    .trim()
    .split(/\s+/)
    .map((item) => item.trim())
    .filter(Boolean)
  const normalized = { id, name, command, args, enabled: Boolean(input?.enabled) }
  for (const field of props.extraFields) {
    normalized[field.key] = String(input?.[field.key] ?? field.defaultValue ?? '').trim()
  }
  return normalized
}

const isEditing = computed(() => editingIndex.value >= 0)
const normalizedQuery = computed(() => String(query.value || '').trim().toLowerCase())

const filteredRows = computed(() => {
  const base = localItems.value.map((item, sourceIndex) => ({ item, sourceIndex }))
  if (!normalizedQuery.value) return base
  return base.filter(({ item }) => {
    const values = [
      item.id,
      item.name,
      item.command,
      ...(Array.isArray(item.args) ? item.args : []),
      ...props.extraFields.map((field) => item[field.key]),
    ]
    return values.some((value) => String(value ?? '').toLowerCase().includes(normalizedQuery.value))
  })
})

const totalPages = computed(() => Math.max(1, Math.ceil(filteredRows.value.length / props.pageSize)))
const pageRangeText = computed(() => {
  if (filteredRows.value.length === 0) return '0-0'
  const start = (currentPage.value - 1) * props.pageSize + 1
  const end = Math.min(filteredRows.value.length, currentPage.value * props.pageSize)
  return `${start}-${end}`
})
const pagedRows = computed(() => {
  const start = (currentPage.value - 1) * props.pageSize
  return filteredRows.value.slice(start, start + props.pageSize)
})

watch(filteredRows, () => {
  if (currentPage.value > totalPages.value) currentPage.value = totalPages.value
})

function startCreate() {
  editingIndex.value = -1
  draft.value = createEmptyItem()
  modalOpen.value = true
}

function startEdit(index) {
  editingIndex.value = index
  draft.value = toDraft(localItems.value[index])
  modalOpen.value = true
}

function removeItem(index) {
  localItems.value.splice(index, 1)
  emit('update:items', copyItems(localItems.value))
}

function toggleItem(index) {
  const item = localItems.value[index]
  if (!item) return
  item.enabled = !item.enabled
  emit('update:items', copyItems(localItems.value))
}

function applyDraft() {
  const normalized = normalizeDraft(draft.value, localItems.value.length + 1)
  const error = props.validateItem(normalized)
  if (error) {
    emit('error', error)
    return
  }
  if (isEditing.value) {
    localItems.value[editingIndex.value] = normalized
  } else {
    localItems.value.push(normalized)
  }
  emit('update:items', copyItems(localItems.value))
  modalOpen.value = false
  editingIndex.value = -1
  draft.value = createEmptyItem()
}

async function saveAll() {
  await emit('save')
}

function closeModal() {
  modalOpen.value = false
  editingIndex.value = -1
  draft.value = createEmptyItem()
}

function goPrevPage() {
  currentPage.value = Math.max(1, currentPage.value - 1)
}

function goNextPage() {
  currentPage.value = Math.min(totalPages.value, currentPage.value + 1)
}

function clearQuery() {
  query.value = ''
}
</script>

<template>
  <section class="qq-chat-workspace">
    <header class="qq-chat-header qq-settings-header">
      <div class="min-w-0 flex-1">
        <h2 class="qq-chat-title">{{ title }}</h2>
        <p class="qq-sidebar-subtitle">{{ subtitle }}</p>
      </div>
    </header>
    <div class="qq-settings-body">
      <div class="qq-settings-card qq-crud-panel">
        <div class="qq-crud-query">
          <div class="qq-search-box qq-crud-search">
            <Search class="h-4 w-4" />
            <input v-model="query" type="text" :placeholder="queryPlaceholder" />
          </div>
          <div class="qq-crud-query-actions">
            <button v-if="query" class="qq-secondary-action h-9 px-4" @click="clearQuery">
              清空筛选
            </button>
            <button class="qq-secondary-action h-9 px-4" @click="startCreate">
              <Plus class="h-4 w-4" />
              <span>新增</span>
            </button>
            <button class="qq-primary-action h-9 px-4" @click="saveAll">
              <Save class="h-4 w-4" />
              <span>{{ saveLabel }}</span>
            </button>
          </div>
        </div>

        <div class="qq-crud-table-wrap">
          <table class="qq-crud-table">
            <thead>
              <tr>
                <th>ID</th>
                <th>名称</th>
                <th v-for="field in extraFields" :key="field.key">{{ field.label }}</th>
                <th>命令</th>
                <th>参数</th>
                <th>启用</th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              <tr v-if="pagedRows.length === 0">
                <td class="qq-crud-empty" :colspan="6 + extraFields.length">
                  <div>{{ emptyText }}</div>
                  <button class="qq-secondary-action h-8 px-3 mt-2" @click="startCreate">立即新增</button>
                </td>
              </tr>
              <tr v-for="(row, pageIndex) in pagedRows" :key="`${row.item.id}_${pageIndex}`">
                <td>{{ row.item.id }}</td>
                <td>{{ row.item.name }}</td>
                <td v-for="field in extraFields" :key="`${row.item.id}_${field.key}`">{{ row.item[field.key] }}</td>
                <td>{{ row.item.command }}</td>
                <td>{{ (row.item.args || []).join(' ') }}</td>
                <td>
                  <span class="qq-session-pill" :class="{ 'is-subagent': row.item.enabled }">{{ row.item.enabled ? '是' : '否' }}</span>
                </td>
                <td>
                  <div class="qq-crud-row-actions">
                    <button class="qq-icon-button" aria-label="编辑条目" @click="startEdit(row.sourceIndex)">
                      <Pencil class="h-4 w-4" />
                    </button>
                    <button class="qq-icon-button" aria-label="切换启用状态" @click="toggleItem(row.sourceIndex)">
                      <Power v-if="row.item.enabled" class="h-4 w-4" />
                      <PowerOff v-else class="h-4 w-4" />
                    </button>
                    <button class="qq-icon-button qq-crud-danger" aria-label="删除条目" @click="removeItem(row.sourceIndex)">
                      <Trash2 class="h-4 w-4" />
                    </button>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>

        <div class="qq-crud-pagination">
          <span>共 {{ filteredRows.length }} 条，当前 {{ pageRangeText }}</span>
          <div class="qq-crud-pagination-actions">
            <button class="qq-secondary-action h-8 px-3" :disabled="currentPage <= 1" @click="goPrevPage">上一页</button>
            <span>{{ currentPage }} / {{ totalPages }}</span>
            <button class="qq-secondary-action h-8 px-3" :disabled="currentPage >= totalPages" @click="goNextPage">下一页</button>
          </div>
        </div>
      </div>
    </div>

    <div v-if="modalOpen" class="qq-modal-backdrop" @click.self="closeModal">
      <div class="qq-modal" role="dialog" aria-modal="true">
        <div class="qq-modal-header">
          <h3 class="qq-modal-title">{{ isEditing ? '编辑数据' : '新增数据' }}</h3>
          <button class="qq-icon-button" type="button" aria-label="关闭弹窗" @click="closeModal">
            <X class="h-4 w-4" />
          </button>
        </div>
        <div class="qq-modal-body">
          <label class="qq-settings-label" for="crudId">ID</label>
          <input id="crudId" v-model="draft.id" type="text" class="qq-settings-input" placeholder="id" />
          <label class="qq-settings-label" for="crudName">名称</label>
          <input id="crudName" v-model="draft.name" type="text" class="qq-settings-input" placeholder="名称" />
          <template v-for="field in extraFields" :key="field.key">
            <label class="qq-settings-label" :for="`crud_${field.key}`">{{ field.label }}</label>
            <input :id="`crud_${field.key}`" v-model="draft[field.key]" type="text" class="qq-settings-input" :placeholder="field.placeholder || ''" />
          </template>
          <label class="qq-settings-label" for="crudCommand">Command</label>
          <input id="crudCommand" v-model="draft.command" type="text" class="qq-settings-input" placeholder="command" />
          <label class="qq-settings-label" for="crudArgs">Args（空格分隔）</label>
          <input id="crudArgs" v-model="draft.argsText" type="text" class="qq-settings-input" placeholder="--flag value" />
          <label class="qq-settings-label" for="crudEnabled">启用状态</label>
          <label class="qq-crud-checkbox">
            <input id="crudEnabled" v-model="draft.enabled" type="checkbox" />
            <span>{{ draft.enabled ? '启用' : '禁用' }}</span>
          </label>
        </div>
        <div class="qq-modal-actions">
          <button class="qq-secondary-action h-8 px-3 text-sm" type="button" @click="closeModal">取消</button>
          <button class="qq-primary-action h-8 px-3 text-sm" type="button" @click="applyDraft">{{ isEditing ? '更新' : '新增' }}</button>
        </div>
      </div>
    </div>
  </section>
</template>
