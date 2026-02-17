export type SessionStatus = 'working' | 'waiting' | 'idle' | 'disconnected' | string

export interface WindowInfo {
  id: string
  name: string
  status: SessionStatus
}

export interface ActionMessage {
  label: string
  keys: string
}

export interface OutputMessage {
  type: 'output'
  window: string
  text: string
  class?: string
  actions?: ActionMessage[]
  id?: string
  ts?: number
}

export interface TerminalDataMessage {
  type: 'terminal_data'
  window: string
  text: string
}

export interface WindowsMessage {
  type: 'windows'
  list: WindowInfo[]
}

export interface StatusMessage {
  type: 'status'
  window: string
  status: SessionStatus
}

export interface ErrorMessage {
  type: 'error'
  message: string
}

export interface ProjectEventMessage {
  type: 'project_event'
  project_id: string
  event: string
  data?: unknown
  ts?: number
}

export type ServerMessage =
  | OutputMessage
  | TerminalDataMessage
  | WindowsMessage
  | StatusMessage
  | ProjectEventMessage
  | ErrorMessage

export interface ClientMessage {
  type: 'input' | 'terminal_input' | 'terminal_resize' | 'new_window' | 'kill_window'
  window?: string
  keys?: string
  cols?: number
  rows?: number
  name?: string
}

export interface OrchestratorClientMessage {
  type: 'chat'
  project_id: string
  message: string
}

export interface OrchestratorHistoryMessage {
  id: string
  project_id: string
  role: string
  content: string
  created_at: string
}

export interface OrchestratorServerTokenMessage {
  type: 'token'
  text?: string
}

export interface OrchestratorServerToolCallMessage {
  type: 'tool_call'
  name?: string
  args?: Record<string, unknown>
}

export interface OrchestratorServerToolResultMessage {
  type: 'tool_result'
  result?: unknown
}

export interface OrchestratorServerDoneMessage {
  type: 'done'
}

export interface OrchestratorServerErrorMessage {
  type: 'error'
  error?: string
}

export type OrchestratorServerMessage =
  | OrchestratorServerTokenMessage
  | OrchestratorServerToolCallMessage
  | OrchestratorServerToolResultMessage
  | OrchestratorServerDoneMessage
  | OrchestratorServerErrorMessage

export interface Project {
  id: string
  name: string
  repo_path: string
  status: string
  playbook?: string
  created_at: string
  updated_at: string
}

export interface Task {
  id: string
  project_id: string
  title: string
  description: string
  status: string
  depends_on: string[]
  worktree_id?: string
  spec_path?: string
  created_at: string
  updated_at: string
}

export interface Session {
  id: string
  task_id?: string
  tmux_session_name: string
  tmux_window_id?: string
  agent_type: string
  role: string
  status: string
  human_attached: boolean
  created_at: string
  last_activity_at: string
}
