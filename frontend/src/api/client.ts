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
  timestamp: string
}

export function getSessionOutput<T = SessionOutputLine[]>(sessionID: string, lines = 400) {
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
