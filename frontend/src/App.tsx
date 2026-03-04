import { createContext, useContext, useEffect, useMemo, useState } from 'react'
import { getToken } from './api/client'
import type { ClientMessage, ServerMessage, WindowInfo } from './api/types'
import { useWebSocket } from './hooks/useWebSocket'

// ---------------------------------------------------------------------------
// App modes
// ---------------------------------------------------------------------------

export type AppMode = 'workspace' | 'demands' | 'settings'

// ---------------------------------------------------------------------------
// Context shape
// ---------------------------------------------------------------------------

interface AppContextValue {
  // Auth & connection
  token: string
  connected: boolean
  connectionStatus: 'connected' | 'connecting' | 'disconnected'
  lastMessage: ServerMessage | null
  send: (message: ClientMessage) => boolean

  // Window state (kept for backward compat with existing Workspace/TerminalGrid)
  windows: WindowInfo[]
  activeWindow: string | null
  unreadByWindow: Record<string, number>
  setActiveWindow: (windowID: string) => void

  // Project state
  selectedProjectID: string | null
  setSelectedProjectID: (id: string | null) => void

  // Mode
  mode: AppMode
  setMode: (mode: AppMode) => void
}

const AppContext = createContext<AppContextValue | null>(null)

export function useAppContext(): AppContextValue {
  const value = useContext(AppContext)
  if (!value) {
    throw new Error('useAppContext must be used within AppContext provider')
  }
  return value
}

// ---------------------------------------------------------------------------
// Placeholder components (will be replaced in Phase 3B)
// ---------------------------------------------------------------------------

function AppSidebar() {
  return (
    <aside className="w-56 bg-bg-secondary border-r border-border flex flex-col shrink-0">
      <div className="px-4 py-3 text-sm font-bold tracking-wider text-text-secondary uppercase">
        agenterm
      </div>
      <div className="flex-1 px-3 py-2 text-xs text-text-secondary">
        Sidebar (TODO)
      </div>
    </aside>
  )
}

// ---------------------------------------------------------------------------
// App
// ---------------------------------------------------------------------------

export default function App() {
  const [token] = useState<string>(() => getToken())
  const ws = useWebSocket(token)

  // ── Window state (backward compat) ──
  const [windows, setWindows] = useState<WindowInfo[]>([])
  const [activeWindow, setActiveWindowState] = useState<string | null>(null)
  const [unreadByWindow, setUnreadByWindow] = useState<Record<string, number>>({})

  const setActiveWindow = (windowID: string) => {
    setActiveWindowState(windowID)
    setUnreadByWindow((prev) => ({ ...prev, [windowID]: 0 }))
  }

  // ── Project state ──
  const [selectedProjectID, setSelectedProjectID] = useState<string | null>(null)

  // ── Mode ──
  const [mode, setMode] = useState<AppMode>('workspace')

  // ── Handle WebSocket messages ──
  useEffect(() => {
    if (!ws.lastMessage) {
      return
    }

    if (ws.lastMessage.type === 'windows') {
      const windowList = Array.isArray(ws.lastMessage.list) ? ws.lastMessage.list : []
      setWindows(windowList)

      setActiveWindowState((current) => {
        if (!current && windowList.length > 0) {
          return windowList[0].id
        }
        if (current && windowList.some((item) => item.id === current)) {
          return current
        }
        return windowList.length > 0 ? windowList[0].id : null
      })

      setUnreadByWindow((prev) => {
        const next: Record<string, number> = {}
        for (const item of windowList) {
          next[item.id] = prev[item.id] ?? 0
        }
        return next
      })
      return
    }

    if (ws.lastMessage.type === 'status') {
      setWindows((prev) =>
        prev.map((item) =>
          item.id === ws.lastMessage.window ? { ...item, status: ws.lastMessage.status } : item,
        ),
      )
      return
    }

    if (ws.lastMessage.type === 'output') {
      const windowID = ws.lastMessage.window
      if (windowID && windowID !== activeWindow) {
        setUnreadByWindow((prev) => ({ ...prev, [windowID]: (prev[windowID] ?? 0) + 1 }))
      }
    }
  }, [activeWindow, ws.lastMessage])

  // ── Context value ──
  const value = useMemo<AppContextValue>(
    () => ({
      token,
      connected: ws.connected,
      connectionStatus: ws.connectionStatus,
      lastMessage: ws.lastMessage,
      send: ws.send,
      windows,
      activeWindow,
      unreadByWindow,
      setActiveWindow,
      selectedProjectID,
      setSelectedProjectID,
      mode,
      setMode,
    }),
    [
      token,
      ws.connected,
      ws.connectionStatus,
      ws.lastMessage,
      ws.send,
      windows,
      activeWindow,
      unreadByWindow,
      selectedProjectID,
      mode,
    ],
  )

  // ── Mode button helper ──
  const modeButton = (m: AppMode, label: string) => (
    <button
      onClick={() => setMode(m)}
      className={`px-3 py-1 text-sm rounded transition-colors ${
        mode === m
          ? 'bg-accent text-white'
          : 'text-text-secondary hover:text-text-primary hover:bg-bg-tertiary'
      }`}
    >
      {label}
    </button>
  )

  return (
    <AppContext.Provider value={value}>
      <div className="flex h-screen bg-bg-primary text-text-primary">
        <AppSidebar />
        <main className="flex-1 flex flex-col overflow-hidden">
          <div className="flex items-center gap-2 px-4 py-2 border-b border-border">
            {modeButton('workspace', 'Workspace')}
            {modeButton('demands', 'Demands')}
            {modeButton('settings', 'Settings')}
            <div className="ml-auto flex items-center gap-2 text-xs">
              <span
                className={`inline-block w-2 h-2 rounded-full ${
                  ws.connectionStatus === 'connected'
                    ? 'bg-status-working'
                    : ws.connectionStatus === 'connecting'
                      ? 'bg-status-waiting'
                      : 'bg-status-error'
                }`}
              />
              <span className="text-text-secondary">{ws.connectionStatus}</span>
            </div>
          </div>
          <div className="flex-1 overflow-hidden">
            {mode === 'workspace' && (
              <div className="flex items-center justify-center h-full text-text-secondary">
                Workspace (TODO)
              </div>
            )}
            {mode === 'demands' && (
              <div className="flex items-center justify-center h-full text-text-secondary">
                Demands (TODO)
              </div>
            )}
            {mode === 'settings' && (
              <div className="flex items-center justify-center h-full text-text-secondary">
                Settings (TODO)
              </div>
            )}
          </div>
        </main>
      </div>
    </AppContext.Provider>
  )
}
