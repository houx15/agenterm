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

function parseApprovalRequiredResult(
  result: unknown,
): { text: string; confirmationOptions: MessageActionOption[] } | null {
  if (!result || typeof result !== 'object' || Array.isArray(result)) {
    return null
  }
  const payload = result as Record<string, unknown>
  if (String(payload.error ?? '').trim() !== 'approval_required') {
    return null
  }
  const reason = String(payload.reason ?? '').trim()
  const text = reason
    ? `Approval needed before execution.\n${reason}\nProceed with the proposed operations?`
    : 'Approval needed before execution. Proceed with the proposed operations?'
  return {
    text,
    confirmationOptions: [
      { label: 'Confirm', value: 'Approved. Execute now.' },
      { label: 'Modify Plan', value: 'Modify plan before execution.' },
      { label: 'Cancel', value: 'Cancel execution for now.' },
    ],
  }
}

function parseStageToolBlockedResult(
  result: unknown,
): { text: string; confirmationOptions: MessageActionOption[] } | null {
  if (!result || typeof result !== 'object' || Array.isArray(result)) {
    return null
  }
  const payload = result as Record<string, unknown>
  if (String(payload.error ?? '').trim() !== 'stage_tool_not_allowed') {
    return null
  }
  const stage = String(payload.stage ?? '').trim() || 'current'
  const tool = String(payload.tool ?? '').trim() || 'requested tool'
  const reason = String(payload.reason ?? '').trim()
  const text = reason
    ? `Execution policy blocked this action.\n${reason}\nPlease confirm whether to continue with a ${stage}-compatible operation.`
    : `Execution policy blocked ${tool} for ${stage} stage. Please adjust the plan.`
  return {
    text,
    confirmationOptions: [
      { label: 'Adjust Plan', value: `Adjust plan for ${stage} stage and retry.` },
      { label: 'Move Stage', value: `Propose transition to next stage before using ${tool}.` },
      { label: 'Cancel', value: 'Cancel this operation.' },
    ],
  }
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

type AssistantEnvelope = {
  discussion?: string
  commands?: string[]
  confirmation?: {
    needed?: boolean
    prompt?: string
  }
}

function parseAssistantEnvelope(text: string): AssistantEnvelope | null {
  const raw = text.trim()
  if (!raw.startsWith('{')) {
    return null
  }
  try {
    const parsed = JSON.parse(raw) as unknown
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
      return null
    }
    return parsed as AssistantEnvelope
  } catch {
    return null
  }
}

function asStringArray(value: unknown): string[] {
  if (!Array.isArray(value)) {
    return []
  }
  return value.filter((item): item is string => typeof item === 'string').map((item) => item.trim()).filter(Boolean)
}

function normalizeAssistantMessage(
  rawText: string,
): { text: string; status: 'discussion' | 'confirmation'; confirmationOptions?: MessageActionOption[] } {
  const envelope = parseAssistantEnvelope(rawText)
  if (!envelope) {
    const fallbackOptions = buildConfirmationOptions(rawText)
    return {
      text: rawText,
      status: fallbackOptions ? 'confirmation' : 'discussion',
      confirmationOptions: fallbackOptions,
    }
  }

  const discussion = typeof envelope.discussion === 'string' ? envelope.discussion.trim() : ''
  const commands = asStringArray(envelope.commands)
  const confirmationNeeded = Boolean(envelope.confirmation?.needed)
  const confirmationPrompt = typeof envelope.confirmation?.prompt === 'string' ? envelope.confirmation.prompt.trim() : ''

  const lines: string[] = []
  if (discussion) {
    lines.push(discussion)
  }
  if (commands.length > 0) {
    lines.push('Planned commands:')
    for (const command of commands) {
      lines.push(`- ${command}`)
    }
  }
  if (confirmationNeeded && confirmationPrompt) {
    lines.push(confirmationPrompt)
  }

  const text = lines.join('\n').trim() || rawText
  if (!confirmationNeeded) {
    return {
      text,
      status: 'discussion',
    }
  }
  return {
    text,
    status: 'confirmation',
    confirmationOptions: [
      { label: 'Confirm', value: 'Confirm' },
      { label: 'Modify', value: 'Modify plan' },
      { label: 'Cancel', value: 'Cancel' },
    ],
  }
}

function toHistorySessionMessage(item: OrchestratorHistoryMessage): SessionMessage {
  const role = item.role === 'user' ? 'user' : 'assistant'
  const text = item.content ?? ''
  const normalized = role === 'assistant' ? normalizeAssistantMessage(text) : null
  return createMessage({
    id: item.id,
    text: normalized?.text ?? text,
    className: role === 'user' ? 'input' : 'output',
    role,
    kind: 'text',
    status: normalized?.status ?? 'discussion',
    isUser: role === 'user',
    timestamp: Date.parse(item.created_at) || Date.now(),
    confirmationOptions: normalized?.confirmationOptions,
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
          status: 'discussion',
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
        const normalized = normalizeAssistantMessage(item.text)
        return {
          ...item,
          text: normalized.text,
          confirmationOptions: normalized.confirmationOptions,
          status: normalized.status,
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
      finishActiveAssistant()
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
      finishActiveAssistant()
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
            status: 'command',
            timestamp: Date.now(),
          }),
        )
        return
      }

      if (parsed.type === 'tool_result') {
        flushBufferedTokens()
        const approvalPrompt = parseApprovalRequiredResult(parsed.result)
        if (approvalPrompt) {
          addMessage(
            createMessage({
              id: `approval-required-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`,
              text: approvalPrompt.text,
              className: 'prompt',
              role: 'assistant',
              kind: 'text',
              status: 'confirmation',
              confirmationOptions: approvalPrompt.confirmationOptions,
              timestamp: Date.now(),
            }),
          )
          return
        }
        const stageBlockedPrompt = parseStageToolBlockedResult(parsed.result)
        if (stageBlockedPrompt) {
          addMessage(
            createMessage({
              id: `stage-tool-blocked-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`,
              text: stageBlockedPrompt.text,
              className: 'prompt',
              role: 'assistant',
              kind: 'text',
              status: 'confirmation',
              confirmationOptions: stageBlockedPrompt.confirmationOptions,
              timestamp: Date.now(),
            }),
          )
          return
        }
        addMessage(
          createMessage({
            id: `tool-result-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`,
            text: buildToolResultText(parsed.result),
            className: 'code',
            role: 'tool',
            kind: 'tool_result',
            status: 'command',
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
            status: 'command',
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
          status: 'discussion',
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
