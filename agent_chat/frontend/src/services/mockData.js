const now = new Date('2026-05-09T10:30:00+08:00').toISOString()

export const mockConversations = [
  { id: 'sess_main_20260509_001', type: 'main', title: 'session 事件持久化', subtitle: '记录 tool call / result / approval 摘要', unreadCount: 2, status: 'waiting_approval', updatedAt: now, avatar: 'agent', model: 'gpt-5.4' },
  { id: 'subsess_review_20260509_001', type: 'subagent', title: 'Review Worker', subtitle: '检查持久化边界与敏感输出策略', unreadCount: 0, status: 'completed', updatedAt: now, avatar: 'subagent', parentSessionId: 'sess_main_20260509_001', skill: 'security-auditor' },
  { id: 'sess_ui_20260509_002', type: 'main', title: '桌面聊天 UI', subtitle: 'Wails + Vue + Pinia 初版骨架', unreadCount: 0, status: 'running', updatedAt: now, avatar: 'workspace', model: 'gpt-5.4' },
]

export const mockMessages = [
  { id: 'msg_1', sessionId: 'sess_main_20260509_001', role: 'user', kind: 'message', content: '为 session 增加最小可用的事件/运行摘要持久化能力。', createdAt: now },
  { id: 'msg_2', sessionId: 'sess_main_20260509_001', role: 'assistant', kind: 'message', content: '我会先记录必要元信息，避免把敏感大输出落盘。', createdAt: now },
  { id: 'tool_1', sessionId: 'sess_main_20260509_001', kind: 'tool_call', toolName: 'shell', status: 'completed', durationMs: 86, summary: '读取 session store 相关文件', safeMeta: { command: 'rg session', outputBytes: 18432, outputHash: 'sha256:4b7c...91af', persistedOutput: false }, createdAt: now },
  { id: 'approval_1', sessionId: 'sess_main_20260509_001', kind: 'approval', title: '写入 session summary', status: 'pending', decision: 'pending', summary: '允许写入摘要索引，不允许保存完整 tool result。', createdAt: now },
  { id: 'run_1', sessionId: 'sess_main_20260509_001', kind: 'subagent_run', subSessionId: 'subsess_review_20260509_001', parentSessionId: 'sess_main_20260509_001', task: '安全审查事件持久化方案', status: 'completed', summary: '未发现敏感大输出落盘路径。', eventCount: 12, createdAt: now },
  { id: 'msg_sub_1', sessionId: 'subsess_review_20260509_001', role: 'assistant', kind: 'message', content: 'subagent 使用独立会话 ID：subsess_review_20260509_001。', createdAt: now },
  { id: 'msg_ui_1', sessionId: 'sess_ui_20260509_002', role: 'assistant', kind: 'message', content: '正在生成三栏桌面聊天布局。', createdAt: now },
]

export const mockRuns = [
  { id: 'run_main_1', sessionId: 'sess_main_20260509_001', status: 'waiting_approval', label: '等待审批', startedAt: now },
  { id: 'run_sub_1', sessionId: 'subsess_review_20260509_001', parentSessionId: 'sess_main_20260509_001', status: 'completed', label: '安全审查完成', startedAt: now, completedAt: now },
]

export const mockApprovals = [
  { id: 'approval_1', sessionId: 'sess_main_20260509_001', decision: 'pending', actor: 'user', summary: '允许保存摘要元信息', createdAt: now },
]

export const mockSkills = [
  { id: 'security-auditor', name: 'security-auditor', description: '审查敏感输出、权限与持久化边界。' },
  { id: 'frontend-design', name: 'frontend-design', description: '生成高质量桌面端界面。' },
]

export const mockAuditEvents = [
  { id: 'audit_1', sessionId: 'sess_main_20260509_001', type: 'tool_result_summary', level: 'info', summary: '仅保存 outputBytes/outputHash/persistedOutput=false。', createdAt: now },
  { id: 'audit_2', sessionId: 'sess_main_20260509_001', type: 'approval_requested', level: 'notice', summary: '等待用户审批写入运行摘要。', createdAt: now },
]
