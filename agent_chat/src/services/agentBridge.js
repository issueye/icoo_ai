import { mockApprovals, mockAuditEvents, mockConversations, mockMessages, mockRuns, mockSkills } from './mockData'

function getWailsService() {
  return globalThis?.go?.bridge?.AgentService ?? globalThis?.wails?.AgentService ?? null
}

async function callOrMock(method, mockValue, ...args) {
  const service = getWailsService()
  if (service && typeof service[method] === 'function') {
    return service[method](...args)
  }
  return structuredClone(mockValue)
}

export const agentBridge = {
  listConversations: () => callOrMock('ListConversations', mockConversations),
  listMessages: (sessionId) => callOrMock('ListMessages', mockMessages.filter((item) => item.sessionId === sessionId), sessionId),
  listRuns: () => callOrMock('ListRuns', mockRuns),
  listApprovals: () => callOrMock('ListApprovals', mockApprovals),
  listSkills: () => callOrMock('ListSkills', mockSkills),
  listAuditEvents: () => callOrMock('ListAuditEvents', mockAuditEvents),
}
