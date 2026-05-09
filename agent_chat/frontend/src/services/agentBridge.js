import { mockApprovals, mockAuditEvents, mockConversations, mockMessages, mockRuns, mockSkills } from './mockData'
import { AgentService as generatedAgentService } from '@/bindings/github.com/icoo-ai/icoo-ai/agent_chat/internal/bridge'

const mockState = {
  conversations: structuredClone(mockConversations),
  messages: structuredClone(mockMessages),
  runs: structuredClone(mockRuns),
  approvals: structuredClone(mockApprovals),
  auditEvents: structuredClone(mockAuditEvents),
  skills: structuredClone(mockSkills),
}

function getWailsService() {
  const hasHostRuntime = Boolean(globalThis?.chrome?.webview || globalThis?.webkit?.messageHandlers || globalThis?.go || globalThis?.wails)
  if (!hasHostRuntime) return null
  return globalThis?.go?.bridge?.AgentService ?? globalThis?.wails?.AgentService ?? generatedAgentService ?? null
}

async function callOrMock(method, mockValue, ...args) {
  const service = getWailsService()
  if (service && typeof service[method] === 'function') {
    try {
      return await service[method](...args)
    } catch (error) {
      console.warn(`Wails bridge ${method} failed, falling back to mock data`, error)
    }
  }
  return structuredClone(mockValue)
}

function touchConversation(sessionId, patch) {
  const conversation = mockState.conversations.find((item) => item.id === sessionId)
  if (!conversation) return
  Object.assign(conversation, patch, { updatedAt: new Date().toISOString() })
}

function makeMessageId(prefix = 'msg') {
  return `${prefix}_${Date.now()}_${mockState.messages.length + 1}`
}

export const agentBridge = {
  listConversations: () => callOrMock('ListConversations', mockState.conversations),
  loadSession: (sessionId) => callOrMock('LoadSession', mockState.conversations.find((item) => item.id === sessionId), sessionId),
  listMessages: (sessionId) => callOrMock('ListMessages', mockState.messages.filter((item) => item.sessionId === sessionId), sessionId),
  listRuns: () => callOrMock('ListRuns', mockState.runs),
  listApprovals: () => callOrMock('ListApprovals', mockState.approvals),
  listSkills: () => callOrMock('ListSkills', mockState.skills),
  listAuditEvents: () => callOrMock('ListAuditEvents', mockState.auditEvents),
  async newSession(payload = {}) {
    const title = payload.title?.trim() || '新的 Agent 会话'
    const conversation = {
      id: `sess_mock_${Date.now()}`,
      type: 'main',
      title,
      subtitle: '已创建 mock 会话，等待输入',
      unreadCount: 0,
      status: 'idle',
      updatedAt: new Date().toISOString(),
      avatar: 'agent',
      workspaceId: payload.workspaceId ?? 'workspace_current',
      cwd: payload.cwd ?? 'E:/code/issueye/icoo_ai',
      mode: payload.mode ?? 'agent',
      model: payload.model ?? 'gpt-5.4',
    }
    const result = await callOrMock('NewSession', conversation, payload)
    if (!mockState.conversations.some((item) => item.id === result.id)) {
      mockState.conversations.unshift(result)
    }
    return structuredClone(result)
  },
  async prompt(payload) {
    const now = new Date().toISOString()
    const events = [
      { id: makeMessageId('msg_user'), sessionId: payload.sessionId, role: 'user', kind: 'message', content: payload.prompt, status: 'done', createdAt: now },
      { id: makeMessageId('msg_assistant'), sessionId: payload.sessionId, role: 'assistant', kind: 'message', content: 'mock bridge 已收到输入。真实 Runtime 接入后这里会由 agent:event 流式更新。', status: 'done', createdAt: now },
    ]
    const result = await callOrMock('Prompt', events, payload)
    const normalized = Array.isArray(result) ? result : events
    mockState.messages.push(...normalized)
    touchConversation(payload.sessionId, { subtitle: 'mock bridge 已生成响应', status: 'idle' })
    return structuredClone(normalized)
  },
  async cancel(sessionId) {
    const run = { id: `run_cancel_${Date.now()}`, sessionId, status: 'cancelled', label: '运行已取消', startedAt: new Date().toISOString(), completedAt: new Date().toISOString() }
    const result = await callOrMock('Cancel', run, sessionId)
    mockState.runs.push(result)
    touchConversation(sessionId, { subtitle: '运行已取消', status: 'cancelled' })
    return structuredClone(result)
  },
  async decideApproval(payload) {
    const decision = { id: payload.id, sessionId: payload.sessionId, decision: payload.decision, actor: 'user', summary: '用户已处理审批请求', createdAt: new Date().toISOString() }
    const result = await callOrMock('DecideApproval', decision, payload)
    const approval = mockState.approvals.find((item) => item.id === payload.id)
    if (approval) Object.assign(approval, result)
    const event = mockState.messages.find((item) => item.id === payload.id)
    if (event) Object.assign(event, { decision: payload.decision, status: 'decided' })
    mockState.auditEvents.push({ id: `audit_${Date.now()}`, sessionId: payload.sessionId, type: 'approval_decision', level: 'notice', summary: `用户决策：${payload.decision}`, createdAt: new Date().toISOString() })
    touchConversation(payload.sessionId, { subtitle: `审批已处理：${payload.decision}`, status: 'idle' })
    return structuredClone(result)
  },
}
