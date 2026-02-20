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

export interface OrchestratorProgressReport {
  [key: string]: unknown
  phase?: string
  queue_depth?: number
  blocked_count?: number
  active_sessions?: number
  pending_tasks?: number
  completed_tasks?: number
  blockers?: string[]
  review_state?: string
  open_review_issues_total?: number
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
  supports_orchestrator?: boolean
  orchestrator_provider?: 'anthropic' | 'openai' | string
  orchestrator_api_key?: string
  orchestrator_api_base?: string
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

export interface AgentRuntimeAssignment {
  session_id: string
  project_id?: string
  project_name?: string
  task_id?: string
  task_title?: string
  role: string
  status: string
  last_activity_at?: string
}

export interface AgentRuntimeStatusItem {
  agent_id: string
  agent_name: string
  capacity: number
  assigned: number
  orchestrator: number
  busy: number
  idle: number
  overflow: number
  assignments: AgentRuntimeAssignment[]
}

export interface AgentStatusResponse {
  total_configured: number
  total_capacity: number
  total_busy: number
  total_assigned: number
  total_orchestrator: number
  total_idle: number
  items: AgentRuntimeStatusItem[]
}

export interface PlaybookPhase {
  name: string
  agent: string
  role: string
  description: string
}

export interface PlaybookWorkflowRole {
  name: string
  responsibilities: string
  allowed_agents: string[]
  suggested_prompt?: string
  mode?: 'planner' | 'worker' | 'reviewer' | 'tester' | string
  inputs_required?: string[]
  actions_allowed?: string[]
  handoff_to?: string[]
  completion_criteria?: string[]
  outputs_contract?: {
    type?: string
    required?: string[]
  }
  gates?: {
    requires_user_approval?: boolean
    pass_condition?: string
  }
  retry_policy?: {
    max_iterations?: number
    escalate_on?: string[]
  }
}

export interface PlaybookWorkflowStage {
  enabled: boolean
  roles: PlaybookWorkflowRole[]
  stage_policy?: {
    enter_gate?: string
    exit_gate?: string
    max_parallel_worktrees?: number
  }
}

export interface PlaybookWorkflow {
  plan: PlaybookWorkflowStage
  build: PlaybookWorkflowStage
  test: PlaybookWorkflowStage
}

export interface Playbook {
  id: string
  name: string
  description: string
  phases: PlaybookPhase[]
  workflow: PlaybookWorkflow
}

export type DemandPoolStatus = 'captured' | 'triaged' | 'shortlisted' | 'scheduled' | 'done' | 'rejected'

export interface DemandPoolItem {
  id: string
  project_id: string
  title: string
  description: string
  status: DemandPoolStatus | string
  priority: number
  impact: number
  effort: number
  risk: number
  urgency: number
  tags: string[]
  source: string
  created_by: string
  selected_task_id?: string
  notes: string
  created_at: string
  updated_at: string
}
