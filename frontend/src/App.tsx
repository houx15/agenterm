import { createContext, useContext, useEffect, useMemo, useState } from 'react'
import { getToken } from './api/client'
import type { ClientMessage, ServerMessage, WindowInfo } from './api/types'
import { useWebSocket } from './hooks/useWebSocket'
import MobileCompanion from './pages/MobileCompanion'
import Workspace from './components/Workspace'

interface AppContextValue {
  token: string
  connected: boolean
  connectionStatus: 'connected' | 'connecting' | 'disconnected'
  lastMessage: ServerMessage | null
  send: (message: ClientMessage) => boolean
  windows: WindowInfo[]
  activeWindow: string | null
  unreadByWindow: Record<string, number>
  setActiveWindow: (windowID: string) => void
}

const AppContext = createContext<AppContextValue | null>(null)

export function useAppContext(): AppContextValue {
  const value = useContext(AppContext)
  if (!value) {
    throw new Error('useAppContext must be used within AppContext provider')
  }
  return value
}

export default function App() {
  const [token] = useState<string>(() => getToken())
  const ws = useWebSocket(token)

  const [windows, setWindows] = useState<WindowInfo[]>([])
  const [activeWindow, setActiveWindowState] = useState<string | null>(null)
  const [unreadByWindow, setUnreadByWindow] = useState<Record<string, number>>({})

  const setActiveWindow = (windowID: string) => {
    setActiveWindowState(windowID)
    setUnreadByWindow((prev) => ({ ...prev, [windowID]: 0 }))
  }

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
      setWindows((prev) => prev.map((item) => (item.id === ws.lastMessage.window ? { ...item, status: ws.lastMessage.status } : item)))
      return
    }

    if (ws.lastMessage.type === 'output') {
      const windowID = ws.lastMessage.window
      if (windowID && windowID !== activeWindow) {
        setUnreadByWindow((prev) => ({ ...prev, [windowID]: (prev[windowID] ?? 0) + 1 }))
      }
    }
  }, [activeWindow, ws.lastMessage])

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
    }),
    [token, ws.connected, ws.connectionStatus, ws.lastMessage, ws.send, windows, activeWindow, unreadByWindow],
  )

  return (
    <AppContext.Provider value={value}>
      {window.location.pathname === '/mobile' ? <MobileCompanion /> : <Workspace />}
    </AppContext.Provider>
  )
}
