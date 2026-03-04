import { buildHTTPURL } from './runtime'

const TOKEN_STORAGE_KEY = 'agenterm_token'
const DESKTOP_FALLBACK_TOKEN = 'agenterm-desktop-local'

export interface CreateProjectInput {
  name: string
  repo_path: string
  playbook?: string
  status?: string
}

export interface DirectoryEntry {
  name: string
  path: string
}

export interface ListDirectoriesResponse {
  path: string
  parent?: string
  directories: DirectoryEntry[]
}

export function getToken(): string {
  const urlToken = new URLSearchParams(window.location.search).get('token')
  if (urlToken) {
    localStorage.setItem(TOKEN_STORAGE_KEY, urlToken)
    return urlToken
  }
  const envToken = (import.meta.env.VITE_AGENTERM_TOKEN as string | undefined)?.trim()
  if (envToken) {
    localStorage.setItem(TOKEN_STORAGE_KEY, envToken)
    return envToken
  }
  const tauriRuntime = typeof (window as Window & { __TAURI_IPC__?: unknown }).__TAURI_IPC__ !== 'undefined'
  if (tauriRuntime) {
    localStorage.setItem(TOKEN_STORAGE_KEY, DESKTOP_FALLBACK_TOKEN)
    return DESKTOP_FALLBACK_TOKEN
  }
  const stored = localStorage.getItem(TOKEN_STORAGE_KEY)
  if (stored) {
    return stored
  }
  return ''
}

interface RequestOptions extends RequestInit {
  auth?: boolean
}

export async function apiFetch<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const token = getToken()
  const headers = new Headers(options.headers)

  if (options.auth !== false && token) {
    headers.set('Authorization', `Bearer ${token}`)
  }
  if (!headers.has('Content-Type') && options.body && !(options.body instanceof FormData)) {
    headers.set('Content-Type', 'application/json')
  }

  const response = await fetch(buildHTTPURL(path), {
    ...options,
    headers,
  })

  if (!response.ok) {
    const text = await response.text()
    throw new Error(text || `Request failed (${response.status})`)
  }

  if (response.status === 204) {
    return undefined as T
  }

  return (await response.json()) as T
}

export function listProjects<T>(params: { status?: string } = {}) {
  const search = new URLSearchParams()
  if (params.status) {
    search.set('status', params.status)
  }
  const query = search.toString()
  return apiFetch<T>(query ? `/api/projects?${query}` : '/api/projects')
}

interface ListSessionsParams {
  status?: string
  taskID?: string
  projectID?: string
}

export function listSessions<T>(params: ListSessionsParams = {}) {
  const search = new URLSearchParams()
  if (params.status) {
    search.set('status', params.status)
  }
  if (params.taskID) {
    search.set('task_id', params.taskID)
  }
  if (params.projectID) {
    search.set('project_id', params.projectID)
  }
  const query = search.toString()
  return apiFetch<T>(query ? `/api/sessions?${query}` : '/api/sessions')
}

export function listProjectTasks<T>(projectID: string) {
  return apiFetch<T>(`/api/projects/${encodeURIComponent(projectID)}/tasks`)
}

export function createProject<T>(input: CreateProjectInput) {
  return apiFetch<T>('/api/projects', {
    method: 'POST',
    body: JSON.stringify(input),
  })
}

export function getProject<T>(projectID: string) {
  return apiFetch<T>(`/api/projects/${encodeURIComponent(projectID)}`)
}

export function updateProject<T>(projectID: string, input: Record<string, unknown>) {
  return apiFetch<T>(`/api/projects/${encodeURIComponent(projectID)}`, {
    method: 'PATCH',
    body: JSON.stringify(input),
  })
}

export function deleteProject(projectID: string) {
  return apiFetch<void>(`/api/projects/${encodeURIComponent(projectID)}`, {
    method: 'DELETE',
  })
}

export function listDirectories<T = ListDirectoriesResponse>(path?: string) {
  const search = new URLSearchParams()
  if (path && path.trim()) {
    search.set('path', path.trim())
  }
  const query = search.toString()
  return apiFetch<T>(query ? `/api/fs/directories?${query}` : '/api/fs/directories')
}

export interface UpdateProjectOrchestratorInput {
  default_provider?: string
  default_model?: string
  max_parallel?: number
}

export function updateProjectOrchestrator<T>(projectID: string, input: UpdateProjectOrchestratorInput) {
  return apiFetch<T>(`/api/projects/${encodeURIComponent(projectID)}/orchestrator`, {
    method: 'PATCH',
    body: JSON.stringify(input),
  })
}

export function listAgents<T>() {
  return apiFetch<T>('/api/agents')
}

export function listAgentStatuses<T>() {
  return apiFetch<T>('/api/agents/status')
}

export function createAgent<T>(input: unknown) {
  return apiFetch<T>('/api/agents', {
    method: 'POST',
    body: JSON.stringify(input),
  })
}

export function updateAgent<T>(id: string, input: unknown) {
  return apiFetch<T>(`/api/agents/${encodeURIComponent(id)}`, {
    method: 'PUT',
    body: JSON.stringify(input),
  })
}

export function deleteAgent(id: string) {
  return apiFetch<void>(`/api/agents/${encodeURIComponent(id)}`, {
    method: 'DELETE',
  })
}

export function listPlaybooks<T>() {
  return apiFetch<T>('/api/playbooks')
}

export function createPlaybook<T>(input: unknown) {
  return apiFetch<T>('/api/playbooks', {
    method: 'POST',
    body: JSON.stringify(input),
  })
}

export function updatePlaybook<T>(id: string, input: unknown) {
  return apiFetch<T>(`/api/playbooks/${encodeURIComponent(id)}`, {
    method: 'PUT',
    body: JSON.stringify(input),
  })
}

export function deletePlaybook(id: string) {
  return apiFetch<void>(`/api/playbooks/${encodeURIComponent(id)}`, {
    method: 'DELETE',
  })
}

export function listOrchestratorHistory<T>(projectID: string, limit = 50) {
  const params = new URLSearchParams()
  params.set('project_id', projectID)
  params.set('limit', String(limit))
  return apiFetch<T>(`/api/orchestrator/history?${params.toString()}`)
}

export function getOrchestratorReport<T>(projectID: string) {
  const params = new URLSearchParams()
  params.set('project_id', projectID)
  return apiFetch<T>(`/api/orchestrator/report?${params.toString()}`)
}

export function listOrchestratorExceptions<T>(projectID: string, status: 'open' | 'resolved' | 'all' = 'open') {
  const params = new URLSearchParams()
  params.set('status', status)
  return apiFetch<T>(`/api/projects/${encodeURIComponent(projectID)}/orchestrator/exceptions?${params.toString()}`)
}

export function resolveOrchestratorException<T>(projectID: string, exceptionID: string, status: 'resolved' | 'open' = 'resolved') {
  return apiFetch<T>(`/api/projects/${encodeURIComponent(projectID)}/orchestrator/exceptions/${encodeURIComponent(exceptionID)}`, {
    method: 'PATCH',
    body: JSON.stringify({ status }),
  })
}

export function chatDemandOrchestrator<T>(input: { project_id: string; message: string }) {
  return apiFetch<T>('/api/demand-orchestrator/chat', {
    method: 'POST',
    body: JSON.stringify(input),
  })
}

export function listDemandOrchestratorHistory<T>(projectID: string, limit = 50) {
  const params = new URLSearchParams()
  params.set('project_id', projectID)
  params.set('limit', String(limit))
  return apiFetch<T>(`/api/demand-orchestrator/history?${params.toString()}`)
}

export function getDemandOrchestratorReport<T>(projectID: string) {
  const params = new URLSearchParams()
  params.set('project_id', projectID)
  return apiFetch<T>(`/api/demand-orchestrator/report?${params.toString()}`)
}

export interface SessionOutputLine {
  text: string
  class?: string
  timestamp: string
}

export interface SessionOutputResponse {
  lines: SessionOutputLine[]
  summary?: {
    prompt_detected: boolean
    last_class: string
    status: string
  }
}

export function getSessionOutput<T = SessionOutputResponse>(sessionID: string, lines = 400) {
  const params = new URLSearchParams()
  params.set('lines', String(lines))
  return apiFetch<T>(`/api/sessions/${encodeURIComponent(sessionID)}/output?${params.toString()}`)
}

export interface EnqueueSessionCommandInput {
  op: 'send_text' | 'send_key' | 'interrupt' | 'resize' | 'close'
  text?: string
  key?: string
  cols?: number
  rows?: number
}

export function enqueueSessionCommand<T>(sessionID: string, input: EnqueueSessionCommandInput) {
  return apiFetch<T>(`/api/sessions/${encodeURIComponent(sessionID)}/commands`, {
    method: 'POST',
    body: JSON.stringify(input),
  })
}

export function getSessionCommand<T>(sessionID: string, commandID: string) {
  return apiFetch<T>(`/api/sessions/${encodeURIComponent(sessionID)}/commands/${encodeURIComponent(commandID)}`)
}

export function listSessionCommands<T>(sessionID: string, limit = 20) {
  const params = new URLSearchParams()
  params.set('limit', String(limit))
  return apiFetch<T>(`/api/sessions/${encodeURIComponent(sessionID)}/commands?${params.toString()}`)
}

export function getSessionReady<T>(sessionID: string) {
  return apiFetch<T>(`/api/sessions/${encodeURIComponent(sessionID)}/ready`)
}

interface ListDemandPoolParams {
  status?: string
  tag?: string
  q?: string
  limit?: number
  offset?: number
}

export function listDemandPoolItems<T>(projectID: string, params: ListDemandPoolParams = {}) {
  const search = new URLSearchParams()
  if (params.status) {
    search.set('status', params.status)
  }
  if (params.tag) {
    search.set('tag', params.tag)
  }
  if (params.q) {
    search.set('q', params.q)
  }
  if (typeof params.limit === 'number') {
    search.set('limit', String(params.limit))
  }
  if (typeof params.offset === 'number') {
    search.set('offset', String(params.offset))
  }
  const query = search.toString()
  const base = `/api/projects/${encodeURIComponent(projectID)}/demand-pool`
  return apiFetch<T>(query ? `${base}?${query}` : base)
}

export function createDemandPoolItem<T>(projectID: string, input: unknown) {
  return apiFetch<T>(`/api/projects/${encodeURIComponent(projectID)}/demand-pool`, {
    method: 'POST',
    body: JSON.stringify(input),
  })
}

export function updateDemandPoolItem<T>(itemID: string, input: unknown) {
  return apiFetch<T>(`/api/demand-pool/${encodeURIComponent(itemID)}`, {
    method: 'PATCH',
    body: JSON.stringify(input),
  })
}

export function deleteDemandPoolItem(itemID: string) {
  return apiFetch<void>(`/api/demand-pool/${encodeURIComponent(itemID)}`, {
    method: 'DELETE',
  })
}

export function promoteDemandPoolItem<T>(itemID: string, input: unknown = {}) {
  return apiFetch<T>(`/api/demand-pool/${encodeURIComponent(itemID)}/promote`, {
    method: 'POST',
    body: JSON.stringify(input),
  })
}

export function reprioritizeDemandPool<T>(projectID: string, input: { items: Array<{ id: string; priority: number }> }) {
  return apiFetch<T>(`/api/projects/${encodeURIComponent(projectID)}/demand-pool/reprioritize`, {
    method: 'POST',
    body: JSON.stringify(input),
  })
}

// --- Settings ---

export interface AppSettings {
  orchestrator_language: string
}

export function getSettings() {
  return apiFetch<AppSettings>('/api/settings')
}

export function updateSettings(input: Partial<AppSettings>) {
  return apiFetch<AppSettings>('/api/settings', {
    method: 'PUT',
    body: JSON.stringify(input),
  })
}

export interface ASRTranscribeInput {
  appID: string
  accessKey: string
  sampleRate?: number
  audio: Blob
}

export function transcribeASR<T>(input: ASRTranscribeInput) {
  const form = new FormData()
  form.set('app_id', input.appID)
  form.set('access_key', input.accessKey)
  form.set('sample_rate', String(input.sampleRate ?? 16000))
  form.set('audio', input.audio, 'speech.pcm')

  return apiFetch<T>('/api/asr/transcribe', {
    method: 'POST',
    body: form,
  })
}

// ─── Requirements ───

export interface Requirement {
  id: string
  project_id: string
  title: string
  description: string
  priority: number
  status: string
  created_at: string
  updated_at: string
}

export function listRequirements(projectID: string) {
  return apiFetch<Requirement[]>(`/api/projects/${encodeURIComponent(projectID)}/requirements`)
}

export function createRequirement(projectID: string, data: { title: string; description?: string }) {
  return apiFetch<Requirement>(`/api/projects/${encodeURIComponent(projectID)}/requirements`, {
    method: 'POST',
    body: JSON.stringify(data),
  })
}

export function updateRequirement(id: string, data: Partial<Pick<Requirement, 'title' | 'description' | 'status'>>) {
  return apiFetch<Requirement>(`/api/requirements/${encodeURIComponent(id)}`, {
    method: 'PATCH',
    body: JSON.stringify(data),
  })
}

export function deleteRequirement(id: string) {
  return apiFetch<void>(`/api/requirements/${encodeURIComponent(id)}`, { method: 'DELETE' })
}

export function reorderRequirements(projectID: string, ids: string[]) {
  return apiFetch<void>(`/api/projects/${encodeURIComponent(projectID)}/requirements/reorder`, {
    method: 'POST',
    body: JSON.stringify({ ids }),
  })
}

// ─── Planning Sessions ───

export interface PlanningSession {
  id: string
  requirement_id: string
  agent_session_id: string
  status: string
  blueprint: string
  created_at: string
  updated_at: string
}

export function createPlanningSession(requirementID: string) {
  return apiFetch<PlanningSession>(`/api/requirements/${encodeURIComponent(requirementID)}/planning`, {
    method: 'POST',
  })
}

export function getPlanningSession(requirementID: string) {
  return apiFetch<PlanningSession>(`/api/requirements/${encodeURIComponent(requirementID)}/planning`)
}

export function saveBlueprint(planningSessionID: string, blueprint: object) {
  return apiFetch<PlanningSession>(`/api/planning-sessions/${encodeURIComponent(planningSessionID)}/blueprint`, {
    method: 'POST',
    body: JSON.stringify({ blueprint: JSON.stringify(blueprint) }),
  })
}

// ─── Execution ───

export function launchExecution(requirementID: string) {
  return apiFetch<any>(`/api/requirements/${encodeURIComponent(requirementID)}/launch`, {
    method: 'POST',
  })
}

export function transitionStage(requirementID: string, transition: string) {
  return apiFetch<any>(`/api/requirements/${encodeURIComponent(requirementID)}/transition`, {
    method: 'POST',
    body: JSON.stringify({ transition }),
  })
}

// ─── Permission Templates ───

export interface PermissionTemplate {
  id: string
  agent_type: string
  name: string
  config: string
  created_at: string
  updated_at: string
}

export function listPermissionTemplates(agentType?: string) {
  const path = agentType
    ? `/api/permission-templates/${encodeURIComponent(agentType)}`
    : '/api/permission-templates'
  return apiFetch<PermissionTemplate[]>(path)
}

export function createPermissionTemplate(data: { agent_type: string; name: string; config: string }) {
  return apiFetch<PermissionTemplate>('/api/permission-templates', {
    method: 'POST',
    body: JSON.stringify(data),
  })
}

export function updatePermissionTemplate(id: string, data: Partial<PermissionTemplate>) {
  return apiFetch<PermissionTemplate>(`/api/permission-templates/${encodeURIComponent(id)}`, {
    method: 'PUT',
    body: JSON.stringify(data),
  })
}

export function deletePermissionTemplate(id: string) {
  return apiFetch<void>(`/api/permission-templates/${encodeURIComponent(id)}`, { method: 'DELETE' })
}

// ─── Agent Status ───

export interface AgentStatus {
  id: string
  name: string
  max_parallel: number
  busy: number
  idle: number
}

export function getAgentStatuses() {
  return apiFetch<AgentStatus[]>('/api/agents/status')
}
