export type SessionStatus = 'working' | 'waiting' | 'idle' | 'disconnected' | string

export interface WindowInfo {
  id: string
  session_id?: string
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
  type: 'input' | 'terminal_input' | 'terminal_resize' | 'new_session' | 'new_window' | 'kill_window'
  session_id?: string
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

export interface ProjectOrchestratorProfile {
  id: string
  project_id: string
  workflow_id: string
  default_provider: string
  default_model: string
  max_parallel: number
  review_policy: string
  notify_on_blocked: boolean
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

export interface AgentConfig {
  id: string
  name: string
  model?: string
  command: string
  max_parallel_agents?: number
  resume_command?: string
  headless_command?: string
  capabilities: string[]
  languages: string[]
  cost_tier: string
  speed_tier: string
  supports_session_resume: boolean
  supports_headless: boolean
  auto_accept_mode?: string
  notes?: string
}

export interface PlaybookMatch {
  languages: string[]
  project_patterns: string[]
}

export interface PlaybookPhase {
  name: string
  agent: string
  role: string
  description: string
}

export interface Playbook {
  id: string
  name: string
  description: string
  match: PlaybookMatch
  phases: PlaybookPhase[]
  parallelism_strategy: string
}
