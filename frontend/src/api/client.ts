const TOKEN_STORAGE_KEY = 'agenterm_token'

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
