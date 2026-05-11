<script setup>
import { computed, ref, watch } from 'vue'
import { Eye, Pencil, Plus, Power, PowerOff, RefreshCcw, Save, Search, Trash2, X } from 'lucide-vue-next'

const props = defineProps({
  title: { type: String, required: true },
  subtitle: { type: String, default: '' },
  items: { type: Array, default: () => [] },
  loading: { type: Boolean, default: false },
  errorText: { type: String, default: '' },
  emptyText: { type: String, default: '暂无数据，请先新增。' },
  queryPlaceholder: { type: String, default: '按 ID / 名称 / 命令搜索' },
  pageSize: { type: Number, default: 10 },
  extraFields: { type: Array, default: () => [] },
  tableColumns: { type: Array, default: () => [] },
  detailFields: { type: Array, default: () => [] },
  allowCreate: { type: Boolean, default: true },
  allowEdit: { type: Boolean, default: true },
  allowDelete: { type: Boolean, default: true },
  allowToggle: { type: Boolean, default: true },
  showRefresh: { type: Boolean, default: false },
  readonly: { type: Boolean, default: false },
  actionMode: { type: String, default: 'edit' }, // edit | view
  detailTitle: { type: String, default: '详情' },
  modalCreateTitle: { type: String, default: '新增数据' },
  modalEditTitle: { type: String, default: '编辑数据' },
  validateItem: { type: Function, default: () => '' },
})

const emit = defineEmits(['update:items', 'save', 'refresh', 'error'])

const draft = ref(createEmptyItem())
const editingIndex = ref(-1)
const localItems = ref(copyItems(props.items))
const query = ref('')
const currentPage = ref(1)
const modalOpen = ref(false)
const viewingItem = ref(null)

watch(() => props.items, (next) => {
  localItems.value = copyItems(next)
}, { deep: true })

function copyItems(items) {
  return Array.isArray(items) ? items.map((item) => ({ ...item, args: Array.isArray(item.args) ? [...item.args] : [] })) : []
}

function createEmptyItem() {
  const base = { id: '', name: '', command: '', argsText: '', enabled: true }
  for (const field of props.extraFields) base[field.key] = field.defaultValue ?? ''
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
  for (const field of props.extraFields) out[field.key] = item[field.key] ?? field.defaultValue ?? ''
  return out
}

function normalizeDraft(input, sequence = 1) {
  const id = String(input?.id || '').trim() || `item_${sequence}`
  const name = String(input?.name || '').trim() || id
  const command = String(input?.command || '').trim()
  const args = String(input?.argsText || '').trim().split(/\s+/).map((item) => item.trim()).filter(Boolean)
  const normalized = { id, name, command, args, enabled: Boolean(input?.enabled) }
  for (const field of props.extraFields) {
    normalized[field.key] = String(input?.[field.key] ?? field.defaultValue ?? '').trim()
  }
  return normalized
}

const isEditing = computed(() => editingIndex.value >= 0)
const normalizedQuery = computed(() => String(query.value || '').trim().toLowerCase())
const effectiveColumns = computed(() => {
  if (props.tableColumns.length) return props.tableColumns
  return [
    { key: 'id', label: 'ID' },
    { key: 'name', label: '名称' },
    ...props.extraFields.map((field) => ({ key: field.key, label: field.label })),
    { key: 'command', label: '命令' },
    { key: 'args', label: '参数', formatter: (item) => (item.args || []).join(' ') },
    { key: 'enabled', label: '启用', type: 'boolean' },
  ]
})
const hasRowActions = computed(() => props.allowEdit || props.allowDelete || props.allowToggle || props.actionMode === 'view')

const filteredRows = computed(() => {
  const base = localItems.value.map((item, sourceIndex) => ({ item, sourceIndex }))
  if (!normalizedQuery.value) return base
  return base.filter(({ item }) => {
    const values = effectiveColumns.value.map((col) => {
      if (typeof col.formatter === 'function') return col.formatter(item)
      if (col.key === 'args') return (item.args || []).join(' ')
      return item[col.key]
    })
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

function formatCell(item, col) {
  if (typeof col.formatter === 'function') return col.formatter(item)
  if (col.key === 'args') return (item.args || []).join(' ')
  return item[col.key]
}

function startCreate() {
  if (props.readonly || !props.allowCreate) return
  editingIndex.value = -1
  viewingItem.value = null
  draft.value = createEmptyItem()
  modalOpen.value = true
}

function startEdit(index) {
  if (!props.allowEdit) return
  editingIndex.value = index
  viewingItem.value = null
  draft.value = toDraft(localItems.value[index])
  modalOpen.value = true
}

function startView(index) {
  viewingItem.value = localItems.value[index]
  editingIndex.value = -1
  modalOpen.value = true
}

function removeItem(index) {
  if (!props.allowDelete || props.readonly) return
  localItems.value.splice(index, 1)
  emit('update:items', copyItems(localItems.value))
}

function toggleItem(index) {
  if (!props.allowToggle || props.readonly) return
  const item = localItems.value[index]
  if (!item) return
  item.enabled = !item.enabled
  emit('update:items', copyItems(localItems.value))
}

function applyDraft() {
  const normalized = normalizeDraft(draft.value, localItems.value.length + 1)
  const error = props.validateItem(normalized)
  if (error) return emit('error', error)
  if (isEditing.value) localItems.value[editingIndex.value] = normalized
  else localItems.value.push(normalized)
  emit('update:items', copyItems(localItems.value))
  closeModal()
}

async function refreshAll() {
  await emit('refresh')
}

function closeModal() {
  modalOpen.value = false
  editingIndex.value = -1
  viewingItem.value = null
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
            <button v-if="query" class="qq-secondary-action h-9 px-4" @click="clearQuery">清空筛选</button>
            <button v-if="showRefresh" class="qq-primary-action h-9 px-4" :disabled="loading" @click="refreshAll">
              <RefreshCcw class="h-4 w-4" />
              <span>刷新</span>
            </button>
            <button v-if="allowCreate && !readonly" class="qq-secondary-action h-9 px-4" @click="startCreate">
              <Plus class="h-4 w-4" />
              <span>新增</span>
            </button>
          </div>
        </div>

        <div class="qq-crud-table-wrap">
          <table class="qq-crud-table">
            <thead>
              <tr>
                <th v-for="col in effectiveColumns" :key="col.key">{{ col.label }}</th>
                <th v-if="hasRowActions">操作</th>
              </tr>
            </thead>
            <tbody>
              <tr v-if="loading">
                <td class="qq-crud-empty" :colspan="effectiveColumns.length + (hasRowActions ? 1 : 0)">数据加载中...</td>
              </tr>
              <tr v-else-if="errorText">
                <td class="qq-crud-empty" :colspan="effectiveColumns.length + (hasRowActions ? 1 : 0)">{{ errorText }}</td>
              </tr>
              <tr v-else-if="pagedRows.length === 0">
                <td class="qq-crud-empty" :colspan="effectiveColumns.length + (hasRowActions ? 1 : 0)">
                  <div>{{ emptyText }}</div>
                  <button v-if="allowCreate && !readonly" class="qq-secondary-action h-8 px-3 mt-2" @click="startCreate">立即新增</button>
                </td>
              </tr>
              <template v-else>
                <tr v-for="(row, pageIndex) in pagedRows" :key="`${row.item.id}_${pageIndex}`">
                  <td v-for="col in effectiveColumns" :key="`${row.item.id}_${col.key}`">
                    <span v-if="col.type === 'boolean'" class="qq-session-pill" :class="{ 'is-subagent': Boolean(row.item[col.key]) }">
                      {{ row.item[col.key] ? '是' : '否' }}
                    </span>
                    <span v-else>{{ formatCell(row.item, col) }}</span>
                  </td>
                  <td v-if="hasRowActions">
                    <div class="qq-crud-row-actions">
                      <button v-if="actionMode === 'view'" class="qq-icon-button" aria-label="查看详情" @click="startView(row.sourceIndex)">
                        <Eye class="h-4 w-4" />
                      </button>
                      <button v-if="allowEdit && actionMode !== 'view'" class="qq-icon-button" aria-label="编辑条目" @click="startEdit(row.sourceIndex)">
                        <Pencil class="h-4 w-4" />
                      </button>
                      <button v-if="allowToggle" class="qq-icon-button" aria-label="切换启用状态" @click="toggleItem(row.sourceIndex)">
                        <Power v-if="row.item.enabled" class="h-4 w-4" />
                        <PowerOff v-else class="h-4 w-4" />
                      </button>
                      <button v-if="allowDelete" class="qq-icon-button qq-crud-danger" aria-label="删除条目" @click="removeItem(row.sourceIndex)">
                        <Trash2 class="h-4 w-4" />
                      </button>
                    </div>
                  </td>
                </tr>
              </template>
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
          <h3 class="qq-modal-title">
            {{ actionMode === 'view' ? detailTitle : (isEditing ? modalEditTitle : modalCreateTitle) }}
          </h3>
          <button class="qq-icon-button" type="button" aria-label="关闭弹窗" @click="closeModal">
            <X class="h-4 w-4" />
          </button>
        </div>
        <div v-if="actionMode === 'view'" class="qq-modal-body">
          <template v-for="field in detailFields" :key="field.key">
            <label class="qq-settings-label">{{ field.label }}</label>
            <input :value="typeof field.formatter === 'function' ? field.formatter(viewingItem || {}) : (viewingItem?.[field.key] ?? '')" type="text" class="qq-settings-input" readonly />
          </template>
        </div>
        <div v-else class="qq-modal-body">
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
          <button v-if="actionMode !== 'view'" class="qq-primary-action h-8 px-3 text-sm" type="button" @click="applyDraft">{{ isEditing ? '更新' : '新增' }}</button>
        </div>
      </div>
    </div>
  </section>
</template>
