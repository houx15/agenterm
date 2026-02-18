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

export function listSessions<T>() {
  return apiFetch<T>('/api/sessions')
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

export function listOrchestratorHistory<T>(projectID: string, limit = 50) {
  const params = new URLSearchParams()
  params.set('project_id', projectID)
  params.set('limit', String(limit))
  return apiFetch<T>(`/api/orchestrator/history?${params.toString()}`)
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
