import * as AgentService from '@/bindings/github.com/icoo-ai/icoo-ai/agent_chat/internal/bridge/agentservice'
import { Call } from '@wailsio/runtime'

const bridgeServiceMethodPrefix = 'github.com/icoo-ai/icoo-ai/agent_chat/internal/bridge.AgentService.'

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

function callBridgeByName(method, ...args) {
  return Call.ByName(`${bridgeServiceMethodPrefix}${method}`, ...args)
}

export const agentBridge = {
  listAgents: () => callBridgeByName('ListAgents'),
  listConversations: () => getBridgeMethod('ListConversations')(),
  loadSession: (sessionId) => getBridgeMethod('LoadSession')(sessionId),
  listMessages: (sessionId) => getBridgeMethod('ListMessages')(sessionId),
  listRuns: () => getBridgeMethod('ListRuns')(),
  listApprovals: () => getBridgeMethod('ListApprovals')(),
  listSkills: () => getBridgeMethod('ListSkills')(),
  listAuditEvents: () => getBridgeMethod('ListAuditEvents')(),
  getGatewayStatus: () => getBridgeMethod('GetGatewayStatus')(),
  restartGateway: () => getBridgeMethod('RestartGateway')(),
  stopGateway: () => getBridgeMethod('StopGateway')(),
  getAppSettings: () => getBridgeMethod('GetAppSettings')(),
  updateAppSettings: (payload = {}) => getBridgeMethod('UpdateAppSettings')(payload),
  newSession: (payload = {}) => getBridgeMethod('NewSession')(payload),
  connectSession: (payload = {}) => callBridgeByName('ConnectSession', payload),
  disconnectSession: (sessionId) => callBridgeByName('DisconnectSession', sessionId),
  deleteSession: (sessionId) => callBridgeByName('DeleteSession', sessionId),
  prompt: (payload) => getBridgeMethod('Prompt')(requirePromptContent(payload)),
  cancel: (sessionId) => getBridgeMethod('Cancel')(sessionId),
  decideApproval: (payload) => getBridgeMethod('DecideApproval')(payload),
}
