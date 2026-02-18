import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import type { OrchestratorClientMessage, OrchestratorHistoryMessage, OrchestratorServerMessage } from '../api/types'
import type { MessageActionOption, SessionMessage } from '../components/ChatMessage'
import { getToken, listOrchestratorHistory } from '../api/client'

const INITIAL_RECONNECT_DELAY_MS = 1000
const MAX_RECONNECT_DELAY_MS = 30000

function createMessage(partial: Partial<SessionMessage> & Pick<SessionMessage, 'text' | 'className'>): SessionMessage {
  return {
    timestamp: Date.now(),
    ...partial,
  }
}

function summarizeData(value: unknown): string {
  if (value == null) {
    return ''
  }

  if (typeof value === 'string') {
    return value
  }

  try {
    return JSON.stringify(value)
  } catch {
    return String(value)
  }
}

function buildToolCallText(name: string, args?: Record<string, unknown>): string {
  const argsText = summarizeData(args)
  if (!argsText || argsText === '{}') {
    return `[${name}]`
  }
  return `[${name} ${argsText}]`
}

function buildToolResultText(result: unknown): string {
  const text = summarizeData(result)
  if (!text) {
    return '[\u2705 done]'
  }
  return `[\u2705 ${text}]`
}

function buildConfirmationOptions(text: string): MessageActionOption[] | undefined {
  const trimmed = text.trim()
  if (!trimmed.endsWith('?')) {
    return undefined
  }

  if (
    /(create|delete|remove|apply|execute|start|run|proceed|continue|merge|deploy|ship|approve|confirm|should i|do you want)/i.test(
      trimmed,
    )
  ) {
    return [
      { label: 'Confirm', value: 'Confirm' },
      { label: 'Modify', value: 'Modify plan' },
      { label: 'Cancel', value: 'Cancel' },
    ]
  }

  return undefined
}

function toHistorySessionMessage(item: OrchestratorHistoryMessage): SessionMessage {
  const role = item.role === 'user' ? 'user' : 'assistant'
  const text = item.content ?? ''
  return createMessage({
    id: item.id,
    text,
    className: role === 'user' ? 'input' : 'output',
    role,
    kind: 'text',
    isUser: role === 'user',
    timestamp: Date.parse(item.created_at) || Date.now(),
    confirmationOptions: role === 'assistant' ? buildConfirmationOptions(text) : undefined,
  })
}

function mergeHistoryMessages(history: SessionMessage[], current: SessionMessage[]): SessionMessage[] {
  if (current.length === 0) {
    return history
  }

  const existingIDs = new Set(current.map((item) => item.id).filter((id): id is string => Boolean(id)))
  const dedupedHistory = history.filter((item) => !item.id || !existingIDs.has(item.id))
  if (dedupedHistory.length === 0) {
    return current
  }
  return dedupedHistory.concat(current)
}

export function useOrchestratorWS(projectId: string) {
  const token = useMemo(() => getToken(), [])
  const wsRef = useRef<WebSocket | null>(null)
  const connectRef = useRef<() => void>(() => {})
  const reconnectTimerRef = useRef<number | null>(null)
  const reconnectAttemptsRef = useRef<number>(0)
  const shouldReconnectRef = useRef<boolean>(false)

  const rafRef = useRef<number | null>(null)
  const tokenBufferRef = useRef<string[]>([])
  const activeAssistantIDRef = useRef<string | null>(null)

  const [messages, setMessages] = useState<SessionMessage[]>([])
  const [isStreaming, setIsStreaming] = useState(false)
  const [connectionStatus, setConnectionStatus] = useState<'connected' | 'connecting' | 'disconnected'>('disconnected')
  const [lastEvent, setLastEvent] = useState<OrchestratorServerMessage | null>(null)

  const clearReconnectTimer = useCallback(() => {
    if (reconnectTimerRef.current !== null) {
      window.clearTimeout(reconnectTimerRef.current)
      reconnectTimerRef.current = null
    }
  }, [])

  const appendTokenChunk = useCallback((chunk: string) => {
    const activeID = activeAssistantIDRef.current
    if (!activeID) {
      return
    }

    setMessages((prev) =>
      prev.map((item) => {
        if (item.id !== activeID) {
          return item
        }
        return {
          ...item,
          text: `${item.text}${chunk}`,
        }
      }),
    )
  }, [])

  const flushBufferedTokens = useCallback(() => {
    const chunk = tokenBufferRef.current.join('')
    tokenBufferRef.current = []
    if (!chunk) {
      return
    }
    appendTokenChunk(chunk)
  }, [appendTokenChunk])

  const scheduleTokenFlush = useCallback(() => {
    if (rafRef.current !== null) {
      return
    }

    rafRef.current = window.requestAnimationFrame(() => {
      rafRef.current = null
      flushBufferedTokens()
      if (tokenBufferRef.current.length > 0) {
        scheduleTokenFlush()
      }
    })
  }, [flushBufferedTokens])

  const pushAssistantPlaceholder = useCallback(() => {
    const id = `assistant-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`
    activeAssistantIDRef.current = id
    setMessages((prev) =>
      prev.concat(
        createMessage({
          id,
          text: '',
          className: 'output',
          role: 'assistant',
          kind: 'text',
        }),
      ),
    )
  }, [])

  const finishActiveAssistant = useCallback(() => {
    flushBufferedTokens()

    const activeID = activeAssistantIDRef.current
    if (!activeID) {
      setIsStreaming(false)
      return
    }

    setMessages((prev) =>
      prev.map((item) => {
        if (item.id !== activeID) {
          return item
        }
        return {
          ...item,
          confirmationOptions: buildConfirmationOptions(item.text),
        }
      }),
    )

    activeAssistantIDRef.current = null
    setIsStreaming(false)
  }, [flushBufferedTokens])

  const addMessage = useCallback((message: SessionMessage) => {
    setMessages((prev) => prev.concat(message))
  }, [])

  const connect = useCallback(() => {
    if (!projectId || !token || wsRef.current?.readyState === WebSocket.OPEN || wsRef.current?.readyState === WebSocket.CONNECTING) {
      return
    }

    setConnectionStatus('connecting')
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const params = new URLSearchParams()
    params.set('token', token)
    params.set('project_id', projectId)
    const url = `${protocol}//${window.location.host}/ws/orchestrator?${params.toString()}`

    const ws = new WebSocket(url)
    wsRef.current = ws

    ws.onopen = () => {
      reconnectAttemptsRef.current = 0
      setConnectionStatus('connected')
    }

    ws.onclose = () => {
      setConnectionStatus('disconnected')
      wsRef.current = null

      if (!shouldReconnectRef.current || !projectId) {
        return
      }

      const attempts = reconnectAttemptsRef.current + 1
      reconnectAttemptsRef.current = attempts
      const delay = Math.min(INITIAL_RECONNECT_DELAY_MS * Math.pow(2, attempts - 1), MAX_RECONNECT_DELAY_MS)

      clearReconnectTimer()
      reconnectTimerRef.current = window.setTimeout(() => {
        connectRef.current()
      }, delay)
    }

    ws.onerror = () => {
      setConnectionStatus('disconnected')
    }

    ws.onmessage = (event) => {
      let parsed: OrchestratorServerMessage
      try {
        parsed = JSON.parse(event.data) as OrchestratorServerMessage
      } catch {
        return
      }

      setLastEvent(parsed)

      if (parsed.type === 'token') {
        const tokenText = parsed.text ?? ''
        if (!tokenText) {
          return
        }
        tokenBufferRef.current.push(tokenText)
        scheduleTokenFlush()
        return
      }

      if (parsed.type === 'tool_call') {
        flushBufferedTokens()
        addMessage(
          createMessage({
            id: `tool-call-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`,
            text: buildToolCallText(parsed.name ?? 'tool_call', parsed.args),
            className: 'prompt',
            role: 'tool',
            kind: 'tool_call',
            timestamp: Date.now(),
          }),
        )
        return
      }

      if (parsed.type === 'tool_result') {
        flushBufferedTokens()
        addMessage(
          createMessage({
            id: `tool-result-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`,
            text: buildToolResultText(parsed.result),
            className: 'code',
            role: 'tool',
            kind: 'tool_result',
            timestamp: Date.now(),
          }),
        )
        return
      }

      if (parsed.type === 'done') {
        finishActiveAssistant()
        return
      }

      if (parsed.type === 'error') {
        finishActiveAssistant()
        addMessage(
          createMessage({
            id: `orchestrator-error-${Date.now()}`,
            text: parsed.error || 'Orchestrator error',
            className: 'error',
            role: 'error',
            kind: 'error',
            timestamp: Date.now(),
          }),
        )
      }
    }
  }, [addMessage, clearReconnectTimer, finishActiveAssistant, flushBufferedTokens, projectId, scheduleTokenFlush, token])

  connectRef.current = connect

  useEffect(() => {
    shouldReconnectRef.current = true
    connect()
    return () => {
      shouldReconnectRef.current = false
      clearReconnectTimer()
      if (rafRef.current !== null) {
        window.cancelAnimationFrame(rafRef.current)
        rafRef.current = null
      }
      tokenBufferRef.current = []
      activeAssistantIDRef.current = null
      wsRef.current?.close()
      wsRef.current = null
    }
  }, [clearReconnectTimer, connect])

  useEffect(() => {
    let canceled = false

    setMessages([])
    tokenBufferRef.current = []
    activeAssistantIDRef.current = null
    setIsStreaming(false)

    if (!projectId) {
      return () => {
        canceled = true
      }
    }

    void (async () => {
      try {
        const items = await listOrchestratorHistory<OrchestratorHistoryMessage[]>(projectId, 50)
        if (canceled) {
          return
        }
        const historyMessages = items.map(toHistorySessionMessage)
        setMessages((prev) => mergeHistoryMessages(historyMessages, prev))
      } catch {
        // Keep any local in-flight/optimistic chat messages if the history request fails.
      }
    })()

    return () => {
      canceled = true
    }
  }, [projectId])

  const send = useCallback(
    (text: string) => {
      const message = text.trim()
      const ws = wsRef.current
      if (!message || !projectId || !ws || ws.readyState !== WebSocket.OPEN) {
        return false
      }

      addMessage(
        createMessage({
          id: `user-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`,
          text: message,
          className: 'input',
          isUser: true,
          role: 'user',
          kind: 'text',
          timestamp: Date.now(),
        }),
      )

      pushAssistantPlaceholder()
      setIsStreaming(true)

      const payload: OrchestratorClientMessage = {
        type: 'chat',
        project_id: projectId,
        message,
      }
      ws.send(JSON.stringify(payload))
      return true
    },
    [addMessage, projectId, pushAssistantPlaceholder],
  )

  return useMemo(
    () => ({
      messages,
      send,
      isStreaming,
      connectionStatus,
      lastEvent,
    }),
    [connectionStatus, isStreaming, lastEvent, messages, send],
  )
}
