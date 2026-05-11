<script setup>
import { computed, onMounted, ref } from 'vue'
import { Pencil, Plus, Power, PowerOff, Save, Trash2 } from 'lucide-vue-next'
import { useAppStore } from '@/stores/app'

const app = useAppStore()
const servers = ref([])
const editingIndex = ref(-1)
const draft = ref(createEmptyServer())

function createEmptyServer() {
  return {
    id: '',
    name: '',
    command: '',
    argsText: '',
    enabled: true,
  }
}

function toDraft(server = {}) {
  return {
    id: server?.id || '',
    name: server?.name || '',
    command: server?.command || '',
    argsText: Array.isArray(server?.args) ? server.args.join(' ') : '',
    enabled: Boolean(server?.enabled),
  }
}

function normalizeDraftServer(value, fallbackIndex = 1) {
  const id = String(value?.id || '').trim() || `mcp_${fallbackIndex}`
  const name = String(value?.name || '').trim() || id
  const command = String(value?.command || '').trim()
  const args = String(value?.argsText || '')
    .trim()
    .split(/\s+/)
    .map((item) => item.trim())
    .filter(Boolean)
  return {
    id,
    name,
    command,
    args,
    enabled: Boolean(value?.enabled),
  }
}

const disabled = computed(() => app.settingsSaving)
const isEditing = computed(() => editingIndex.value >= 0)

onMounted(async () => {
  await app.loadAppSettings()
  servers.value = Array.isArray(app.mcpServers) ? app.mcpServers.map((item) => ({ ...item })) : []
})

function startCreate() {
  editingIndex.value = -1
  draft.value = createEmptyServer()
}

function startEdit(index) {
  editingIndex.value = index
  draft.value = toDraft(servers.value[index])
}

function removeServer(index) {
  servers.value.splice(index, 1)
}

function toggleServer(index) {
  const current = servers.value[index]
  if (!current) return
  current.enabled = !current.enabled
}

function applyDraft() {
  const normalized = normalizeDraftServer(draft.value, servers.value.length + 1)
  if (!normalized.command) {
    app.pushToast({ type: 'error', message: '请填写 command' })
    return
  }
  if (isEditing.value) {
    servers.value[editingIndex.value] = normalized
  } else {
    servers.value.push(normalized)
  }
  startCreate()
}

async function saveMcpSettings() {
  try {
    await app.saveAppSettings({ mcpServers: servers.value })
    servers.value = Array.isArray(app.mcpServers) ? app.mcpServers.map((item) => ({ ...item })) : []
    app.pushToast({ type: 'success', message: 'MCP 配置保存成功' })
  } catch {
    app.pushToast({ type: 'error', message: app.settingsError || 'MCP 配置保存失败' })
  }
}
</script>

<template>
  <section class="qq-chat-workspace">
    <header class="qq-chat-header qq-settings-header">
      <div class="min-w-0 flex-1">
        <h2 class="qq-chat-title">MCP 管理</h2>
        <p class="qq-sidebar-subtitle">配置 MCP servers（id / name / command / args / enabled）</p>
      </div>
    </header>
    <div class="qq-settings-body">
      <div class="qq-settings-card qq-mcp-panel">
        <div class="qq-mcp-form">
          <label class="qq-settings-label" for="mcpId">ID</label>
          <input id="mcpId" v-model="draft.id" type="text" class="qq-settings-input" placeholder="mcp_server_id" />
          <label class="qq-settings-label" for="mcpName">名称</label>
          <input id="mcpName" v-model="draft.name" type="text" class="qq-settings-input" placeholder="MCP Server" />
          <label class="qq-settings-label" for="mcpCommand">Command</label>
          <input id="mcpCommand" v-model="draft.command" type="text" class="qq-settings-input" placeholder="npx" />
          <label class="qq-settings-label" for="mcpArgs">Args（空格分隔）</label>
          <input id="mcpArgs" v-model="draft.argsText" type="text" class="qq-settings-input" placeholder="-y @modelcontextprotocol/server-filesystem" />
          <div class="qq-mcp-form-actions">
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

        <div class="qq-mcp-list">
          <div v-if="servers.length === 0" class="qq-settings-helper">暂无 MCP server，请先新增。</div>
          <div v-for="(server, index) in servers" :key="`${server.id}_${index}`" class="qq-mcp-item">
            <div class="qq-mcp-item-main">
              <div class="qq-mcp-item-title">{{ server.name }}（{{ server.id }}）</div>
              <div class="qq-mcp-item-subtitle">{{ server.command }} {{ (server.args || []).join(' ') }}</div>
            </div>
            <div class="qq-mcp-item-actions">
              <button class="qq-icon-button" :disabled="disabled" aria-label="编辑条目" @click="startEdit(index)">
                <Pencil class="h-4 w-4" />
              </button>
              <button class="qq-icon-button" :disabled="disabled" aria-label="切换启用状态" @click="toggleServer(index)">
                <Power v-if="server.enabled" class="h-4 w-4" />
                <PowerOff v-else class="h-4 w-4" />
              </button>
              <button class="qq-icon-button qq-mcp-danger" :disabled="disabled" aria-label="删除条目" @click="removeServer(index)">
                <Trash2 class="h-4 w-4" />
              </button>
            </div>
          </div>
        </div>

        <div class="qq-settings-actions qq-mcp-save-row">
          <button class="qq-primary-action h-9 px-4" :disabled="disabled" @click="saveMcpSettings">
            <Save class="h-4 w-4" />
            <span>{{ app.settingsSaving ? '保存中' : '保存 MCP 配置' }}</span>
          </button>
          <span v-if="app.settingsError" class="qq-settings-error">{{ app.settingsError }}</span>
        </div>
      </div>
    </div>
  </section>
</template>
