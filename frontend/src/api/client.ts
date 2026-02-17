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
  if (!headers.has('Content-Type') && options.body) {
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
