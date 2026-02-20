import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import type { ClientMessage, ServerMessage } from '../api/types'

const INITIAL_RECONNECT_DELAY_MS = 1000
const MAX_RECONNECT_DELAY_MS = 30000

export function useWebSocket(token: string) {
  const wsRef = useRef<WebSocket | null>(null)
  const reconnectTimerRef = useRef<number | null>(null)
  const reconnectAttemptsRef = useRef<number>(0)
  const shouldReconnectRef = useRef<boolean>(false)

  const [connected, setConnected] = useState(false)
  const [connectionStatus, setConnectionStatus] = useState<'connected' | 'connecting' | 'disconnected'>('disconnected')
  const [messages, setMessages] = useState<ServerMessage[]>([])
  const [lastMessage, setLastMessage] = useState<ServerMessage | null>(null)
  const isVisibleRef = useRef<boolean>(typeof document !== 'undefined' ? document.visibilityState === 'visible' : true)

  const clearReconnectTimer = useCallback(() => {
    if (reconnectTimerRef.current !== null) {
      window.clearTimeout(reconnectTimerRef.current)
      reconnectTimerRef.current = null
    }
  }, [])

  const connect = useCallback(() => {
    if (!token || wsRef.current?.readyState === WebSocket.OPEN || wsRef.current?.readyState === WebSocket.CONNECTING) {
      return
    }

    setConnectionStatus('connecting')
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const url = `${protocol}//${window.location.host}/ws?token=${encodeURIComponent(token)}`

    const ws = new WebSocket(url)
    wsRef.current = ws

    ws.onopen = () => {
      reconnectAttemptsRef.current = 0
      setConnected(true)
      setConnectionStatus('connected')
    }

    ws.onclose = () => {
      setConnected(false)
      setConnectionStatus('disconnected')
      wsRef.current = null

      if (!shouldReconnectRef.current) {
        return
      }
      if (!isVisibleRef.current) {
        return
      }

      const attempts = reconnectAttemptsRef.current + 1
      reconnectAttemptsRef.current = attempts
      const delay = Math.min(INITIAL_RECONNECT_DELAY_MS * Math.pow(2, attempts - 1), MAX_RECONNECT_DELAY_MS)

      clearReconnectTimer()
      reconnectTimerRef.current = window.setTimeout(() => {
        connect()
      }, delay)
    }

    ws.onerror = () => {
      setConnected(false)
      setConnectionStatus('disconnected')
    }

    ws.onmessage = (event) => {
      try {
        const parsed = JSON.parse(event.data) as ServerMessage
        setLastMessage(parsed)
        setMessages((prev) => [...prev.slice(-999), parsed])
      } catch {
        // Ignore malformed messages from the server.
      }
    }
  }, [clearReconnectTimer, token])

  useEffect(() => {
    shouldReconnectRef.current = true
    connect()
    const onVisibilityChange = () => {
      const visible = document.visibilityState === 'visible'
      isVisibleRef.current = visible
      if (!visible) {
        clearReconnectTimer()
        return
      }
      if (!wsRef.current || wsRef.current.readyState === WebSocket.CLOSED) {
        connect()
      }
    }
    document.addEventListener('visibilitychange', onVisibilityChange)
    return () => {
      shouldReconnectRef.current = false
      document.removeEventListener('visibilitychange', onVisibilityChange)
      clearReconnectTimer()
      wsRef.current?.close()
      wsRef.current = null
    }
  }, [clearReconnectTimer, connect])

  const send = useCallback((message: ClientMessage) => {
    const ws = wsRef.current
    if (!ws || ws.readyState !== WebSocket.OPEN) {
      return false
    }
    ws.send(JSON.stringify(message))
    return true
  }, [])

  return useMemo(
    () => ({ connected, connectionStatus, messages, lastMessage, send }),
    [connected, connectionStatus, messages, lastMessage, send],
  )
}
