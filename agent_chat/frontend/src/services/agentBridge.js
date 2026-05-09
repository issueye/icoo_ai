import * as AgentService from '@/bindings/github.com/icoo-ai/icoo-ai/agent_chat/internal/bridge/agentservice'

function getBridgeMethod(name) {
  const method = AgentService?.[name]
  if (typeof method !== 'function') {
    throw new Error(`Wails bridge method not available: ${name}`)
  }
  return method
}

function requirePromptContent(payload) {
  if (!payload || typeof payload !== 'object') {
    throw new Error('Prompt request payload is required')
  }
  const content = typeof payload.content === 'string' ? payload.content.trim() : ''
  if (!content) {
    throw new Error('PromptRequest.content is required')
  }
  return { ...payload, content }
}

export const agentBridge = {
  listConversations: () => getBridgeMethod('ListConversations')(),
  loadSession: (sessionId) => getBridgeMethod('LoadSession')(sessionId),
  listMessages: (sessionId) => getBridgeMethod('ListMessages')(sessionId),
  listRuns: () => getBridgeMethod('ListRuns')(),
  listApprovals: () => getBridgeMethod('ListApprovals')(),
  listSkills: () => getBridgeMethod('ListSkills')(),
  listAuditEvents: () => getBridgeMethod('ListAuditEvents')(),
  getGatewayStatus: () => getBridgeMethod('GetGatewayStatus')(),
  restartGateway: () => getBridgeMethod('RestartGateway')(),
  getAppSettings: () => getBridgeMethod('GetAppSettings')(),
  updateAppSettings: (payload = {}) => getBridgeMethod('UpdateAppSettings')(payload),
  newSession: (payload = {}) => getBridgeMethod('NewSession')(payload),
  prompt: (payload) => getBridgeMethod('Prompt')(requirePromptContent(payload)),
  cancel: (sessionId) => getBridgeMethod('Cancel')(sessionId),
  decideApproval: (payload) => getBridgeMethod('DecideApproval')(payload),
}
