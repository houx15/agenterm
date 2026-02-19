const TOKEN_STORAGE_KEY = 'agenterm_token'

export interface CreateProjectInput {
  name: string
  repo_path: string
  playbook?: string
  status?: string
}

export function getToken(): string {
  const urlToken = new URLSearchParams(window.location.search).get('token')
  if (urlToken) {
    localStorage.setItem(TOKEN_STORAGE_KEY, urlToken)
    return urlToken
  }
  return localStorage.getItem(TOKEN_STORAGE_KEY) ?? ''
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

  const response = await fetch(path, {
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

export function listProjects<T>() {
  return apiFetch<T>('/api/projects')
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
