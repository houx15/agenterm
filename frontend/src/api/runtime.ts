const DESKTOP_BACKEND_ORIGIN = 'http://127.0.0.1:8765'

function isTauriRuntime(): boolean {
  return typeof (window as Window & { __TAURI_IPC__?: unknown }).__TAURI_IPC__ !== 'undefined'
}

export function backendOrigin(): string {
  const configured = (import.meta.env.VITE_BACKEND_ORIGIN as string | undefined)?.trim()
  if (configured) {
    return configured.replace(/\/+$/, '')
  }
  if (isTauriRuntime()) {
    return DESKTOP_BACKEND_ORIGIN
  }
  return ''
}

export function buildHTTPURL(path: string): string {
  const base = backendOrigin()
  if (!base || /^https?:\/\//i.test(path)) {
    return path
  }
  return `${base}${path}`
}

export function buildWSURL(path: string): string {
  const normalizedPath = path.startsWith('/') ? path : `/${path}`
  const base = backendOrigin()
  if (base) {
    const http = new URL(base)
    const protocol = http.protocol === 'https:' ? 'wss:' : 'ws:'
    return `${protocol}//${http.host}${normalizedPath}`
  }
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  return `${protocol}//${window.location.host}${normalizedPath}`
}
