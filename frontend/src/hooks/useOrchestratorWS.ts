import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import type { OrchestratorClientMessage, OrchestratorHistoryMessage, OrchestratorServerMessage } from '../api/types'
import type { MessageActionOption, SessionMessage } from '../components/ChatMessage'
import { getToken, listOrchestratorHistory } from '../api/client'
import { buildWSURL } from '../api/runtime'
import { orchestratorEventBus } from '../orchestrator/bus'
import { loadProjectTimeline, saveProjectTimeline } from '../orchestrator/replay'
import { sessionMessageToEvent } from '../orchestrator/schema'

const INITIAL_RECONNECT_DELAY_MS = 1000
const MAX_RECONNECT_DELAY_MS = 30000

type AssistantEnvelope = {
  discussion?: string
  commands?: string[]
  state_update?: unknown
  confirmation?: {
    needed?: boolean
    prompt?: string
  }
}

type NormalizedAssistantMessage = {
  text: string
  status: 'discussion' | 'confirmation'
  confirmationOptions?: MessageActionOption[]
  discussion?: string
  commands?: string[]
  stateUpdates?: string[]
  confirmationPrompt?: string
}

type StoredContentBlock = {
  type?: string
  text?: string
}

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

function buildToolCallSummary(name: string, args?: Record<string, unknown>): string {
  const command = String(name || 'tool_call').trim()
  if (!args || Object.keys(args).length === 0) {
    return command
  }
  const argsText = summarizeData(args)
  if (!argsText || argsText === '{}') {
    return command
  }
  return `${command} ${argsText}`
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

function parseToolFailureResult(result: unknown): string {
  if (!result || typeof result !== 'object' || Array.isArray(result)) {
    return ''
  }
  const payload = result as Record<string, unknown>
  const error = String(payload.error ?? '').trim()
  if (!error || error === 'approval_required' || error === 'stage_tool_not_allowed') {
    return ''
  }
  const reason = String(payload.reason ?? '').trim()
  if (reason) {
    return `${error}: ${reason}`
  }
  const hint = String(payload.hint ?? '').trim()
  if (hint) {
    return `${error}: ${hint}`
  }
  return error
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

function sanitizeAssistantText(text: string): string {
  if (!text) {
    return ''
  }
  const stripped = text
    .replace(/\[tool_result:[\s\S]*?\]/gi, '')
    .replace(/\[tool_call:[\s\S]*?\]/gi, '')
    .replace(/\[tool_result:[^\n\r]*/gi, '')
    .replace(/\[tool_call:[^\n\r]*/gi, '')
    .replace(/\[✅[\s\S]*?\]/g, '')
  const cleanedLines = stripped
    .split('\n')
    .map((line) => line.trim())
    .filter((line) => Boolean(line))
    .filter((line) => !/^\s*command\s*$/i.test(line))
    .filter((line) => !/^\s*(tool_result|tool_call|discussion)\s*$/i.test(line))
    .filter((line) => !looksLikeToolPayloadLine(line))
  return cleanedLines.join('\n').trim()
}

function looksLikeToolPayloadLine(line: string): boolean {
  const trimmed = line.trim()
  if (!trimmed || (!trimmed.startsWith('{') && !trimmed.startsWith('['))) {
    return false
  }
  try {
    const parsed = JSON.parse(trimmed) as unknown
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
      return false
    }
    const payload = parsed as Record<string, unknown>
    const keys = Object.keys(payload)
    if (keys.length === 0) {
      return false
    }
    const toolKeys = new Set([
      'error',
      'reason',
      'hint',
      'created_at',
      'updated_at',
      'project_id',
      'task_id',
      'status',
      'id',
      'skills',
    ])
    return keys.some((key) => toolKeys.has(key))
  } catch {
    return false
  }
}

function extractJSONObjectChunks(text: string): string[] {
  const chunks: string[] = []
  const raw = text.trim()
  if (!raw) {
    return chunks
  }

  let depth = 0
  let inString = false
  let escaped = false
  let start = -1

  for (let i = 0; i < raw.length; i += 1) {
    const ch = raw[i]
    if (inString) {
      if (escaped) {
        escaped = false
      } else if (ch === '\\') {
        escaped = true
      } else if (ch === '"') {
        inString = false
      }
      continue
    }

    if (ch === '"') {
      inString = true
      continue
    }
    if (ch === '{') {
      if (depth === 0) {
        start = i
      }
      depth += 1
      continue
    }
    if (ch === '}') {
      if (depth === 0) {
        continue
      }
      depth -= 1
      if (depth === 0 && start >= 0) {
        chunks.push(raw.slice(start, i + 1))
        start = -1
      }
    }
  }
  return chunks
}

function parseAssistantEnvelopes(text: string): AssistantEnvelope[] {
  const raw = text.trim()
  if (!raw) {
    return []
  }
  const chunks = extractJSONObjectChunks(raw)
  if (chunks.length === 0) {
    return []
  }
  const envelopes: AssistantEnvelope[] = []
  for (const chunk of chunks) {
    try {
      const parsed = JSON.parse(chunk) as unknown
      if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
        continue
      }
      if (!isAssistantEnvelopePayload(parsed)) {
        continue
      }
      envelopes.push(parsed as AssistantEnvelope)
    } catch {
      // Skip malformed chunk and keep parsing others.
    }
  }
  return envelopes
}

function isAssistantEnvelopePayload(value: unknown): value is AssistantEnvelope {
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    return false
  }
  const payload = value as Record<string, unknown>
  return 'discussion' in payload || 'commands' in payload || 'state_update' in payload || 'confirmation' in payload
}

function asStringArray(value: unknown): string[] {
  if (!Array.isArray(value)) {
    return []
  }
  return value.filter((item): item is string => typeof item === 'string').map((item) => item.trim()).filter(Boolean)
}

function collectStateUpdates(value: unknown): string[] {
  if (value == null) {
    return []
  }
  if (typeof value === 'string') {
    const text = value.trim()
    return text ? [text] : []
  }
  if (Array.isArray(value)) {
    const items: string[] = []
    for (const entry of value) {
      const text = summarizeData(entry).trim()
      if (text) {
        items.push(text)
      }
    }
    return items
  }
  if (typeof value === 'object') {
    const objectValue = value as Record<string, unknown>
    return Object.keys(objectValue)
      .sort((left, right) => left.localeCompare(right))
      .map((key) => {
        const summary = summarizeData(objectValue[key]).trim()
        if (!summary) {
          return key
        }
        return `${key}: ${summary}`
      })
      .filter(Boolean)
  }
  const fallback = summarizeData(value).trim()
  return fallback ? [fallback] : []
}

function uniqueMerge(items: string[]): string[] {
  const output: string[] = []
  const seen = new Set<string>()
  for (const item of items) {
    const normalized = item.trim()
    if (!normalized) {
      continue
    }
    if (seen.has(normalized)) {
      continue
    }
    seen.add(normalized)
    output.push(normalized)
  }
  return output
}

function buildDisplayText(discussion: string, commands: string[], stateUpdates: string[], confirmationPrompt: string): string {
  const lines: string[] = []
  if (discussion) {
    lines.push(discussion)
  }
  if (commands.length > 0) {
    lines.push('Commands:')
    for (const command of commands) {
      lines.push(`- ${command}`)
    }
  }
  if (stateUpdates.length > 0) {
    lines.push('State updates:')
    for (const update of stateUpdates) {
      lines.push(`- ${update}`)
    }
  }
  if (confirmationPrompt) {
    lines.push(confirmationPrompt)
  }
  return sanitizeAssistantText(lines.join('\n').trim())
}

function normalizeAssistantMessage(rawText: string, draftCommands: string[] = [], draftStateUpdates: string[] = []): NormalizedAssistantMessage {
  const envelopes = parseAssistantEnvelopes(rawText)
  const commandLines: string[] = [...draftCommands]
  const stateLines: string[] = [...draftStateUpdates]
  const discussionParts: string[] = []
  let confirmationNeeded = false
  let confirmationPrompt = ''

  if (envelopes.length === 0) {
    const discussion = sanitizeAssistantText(rawText)
    const commands = uniqueMerge(commandLines)
    const stateUpdates = uniqueMerge(stateLines)
    const text = buildDisplayText(discussion, commands, stateUpdates, '')
    const fallbackOptions = buildConfirmationOptions(text)
    return {
      text,
      status: fallbackOptions ? 'confirmation' : 'discussion',
      confirmationOptions: fallbackOptions,
      discussion,
      commands,
      stateUpdates,
    }
  }

  for (const envelope of envelopes) {
    const discussion = typeof envelope.discussion === 'string' ? envelope.discussion.trim() : ''
    if (discussion) {
      discussionParts.push(discussion)
    }
    commandLines.push(...asStringArray(envelope.commands))
    stateLines.push(...collectStateUpdates(envelope.state_update))

    const needed = Boolean(envelope.confirmation?.needed)
    const prompt = typeof envelope.confirmation?.prompt === 'string' ? envelope.confirmation.prompt.trim() : ''
    if (needed) {
      confirmationNeeded = true
      if (prompt) {
        confirmationPrompt = prompt
      }
    }
  }

  const discussion = sanitizeAssistantText(discussionParts.join('\n').trim())
  const commands = uniqueMerge(commandLines)
  const stateUpdates = uniqueMerge(stateLines)
  const text = buildDisplayText(discussion, commands, stateUpdates, confirmationNeeded ? confirmationPrompt : '')
  if (!confirmationNeeded) {
    return {
      text,
      status: 'discussion',
      discussion,
      commands,
      stateUpdates,
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
    discussion,
    commands,
    stateUpdates,
    confirmationPrompt,
  }
}

function parseHistoryText(item: OrchestratorHistoryMessage): string {
  const raw = (item.message_json ?? '').trim()
  if (!raw) {
    return item.content ?? ''
  }
  try {
    const blocks = JSON.parse(raw) as unknown
    if (!Array.isArray(blocks)) {
      return item.content ?? ''
    }
    const texts = (blocks as StoredContentBlock[])
      .filter((block) => String(block.type ?? '').trim() === 'text')
      .map((block) => (block.text ?? '').trim())
      .filter(Boolean)
    if (texts.length > 0) {
      return texts.join('\n')
    }
    // Stored non-text blocks (tool_use/tool_result) should not be rendered in PM chat history.
    return ''
  } catch {
    // Fall back to summarized content.
  }
  return item.content ?? ''
}

function toHistorySessionMessage(item: OrchestratorHistoryMessage): SessionMessage | null {
  const role = item.role === 'user' ? 'user' : 'assistant'
  const text = parseHistoryText(item)
  if (!text.trim()) {
    return null
  }
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
    discussion: normalized?.discussion,
    commands: normalized?.commands,
    stateUpdates: normalized?.stateUpdates,
    confirmationPrompt: normalized?.confirmationPrompt,
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
  const activeAssistantCommandsRef = useRef<string[]>([])
  const activeAssistantStateUpdatesRef = useRef<string[]>([])
  const lastEmittedMessageRef = useRef<string>('')

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

  const clearActiveAssistantMetadata = useCallback(() => {
    activeAssistantCommandsRef.current = []
    activeAssistantStateUpdatesRef.current = []
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
    clearActiveAssistantMetadata()
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
  }, [clearActiveAssistantMetadata])

  const ensureActiveAssistant = useCallback(() => {
    if (activeAssistantIDRef.current) {
      return
    }
    pushAssistantPlaceholder()
    setIsStreaming(true)
  }, [pushAssistantPlaceholder])

  const appendDraftCommand = useCallback((line: string) => {
    const normalized = line.trim()
    if (!normalized) {
      return
    }
    if (!activeAssistantCommandsRef.current.includes(normalized)) {
      activeAssistantCommandsRef.current.push(normalized)
    }
  }, [])

  const appendDraftStateUpdate = useCallback((line: string) => {
    const normalized = line.trim()
    if (!normalized) {
      return
    }
    if (!activeAssistantStateUpdatesRef.current.includes(normalized)) {
      activeAssistantStateUpdatesRef.current.push(normalized)
    }
  }, [])

  const finishActiveAssistant = useCallback(() => {
    flushBufferedTokens()

    const activeID = activeAssistantIDRef.current
    const draftCommands = [...activeAssistantCommandsRef.current]
    const draftStateUpdates = [...activeAssistantStateUpdatesRef.current]
    if (!activeID) {
      clearActiveAssistantMetadata()
      setIsStreaming(false)
      return
    }

    setMessages((prev) =>
      prev.flatMap((item) => {
        if (item.id !== activeID) {
          return [item]
        }
        const normalized = normalizeAssistantMessage(item.text, draftCommands, draftStateUpdates)
        const hasStructured = Boolean(
          normalized.text.trim() ||
            (normalized.discussion ?? '').trim() ||
            (normalized.commands && normalized.commands.length > 0) ||
            (normalized.stateUpdates && normalized.stateUpdates.length > 0),
        )
        if (!hasStructured) {
          return []
        }
        return [
          {
            ...item,
            text: normalized.text,
            confirmationOptions: normalized.confirmationOptions,
            status: normalized.status,
            discussion: normalized.discussion,
            commands: normalized.commands,
            stateUpdates: normalized.stateUpdates,
            confirmationPrompt: normalized.confirmationPrompt,
          },
        ]
      }),
    )

    activeAssistantIDRef.current = null
    clearActiveAssistantMetadata()
    setIsStreaming(false)
  }, [clearActiveAssistantMetadata, flushBufferedTokens])

  const addMessage = useCallback((message: SessionMessage) => {
    setMessages((prev) => prev.concat(message))
  }, [])

  const connect = useCallback(() => {
    if (!projectId || !token || wsRef.current?.readyState === WebSocket.OPEN || wsRef.current?.readyState === WebSocket.CONNECTING) {
      return
    }

    setConnectionStatus('connecting')
    const params = new URLSearchParams()
    params.set('token', token)
    params.set('project_id', projectId)
    const url = `${buildWSURL('/ws/orchestrator')}?${params.toString()}`

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
        ensureActiveAssistant()
        tokenBufferRef.current.push(tokenText)
        scheduleTokenFlush()
        return
      }

      if (parsed.type === 'tool_call') {
        ensureActiveAssistant()
        flushBufferedTokens()
        appendDraftCommand(buildToolCallSummary(parsed.name ?? 'tool_call', parsed.args))
        return
      }

      if (parsed.type === 'tool_result') {
        ensureActiveAssistant()
        flushBufferedTokens()
        const approvalPrompt = parseApprovalRequiredResult(parsed.result)
        if (approvalPrompt) {
          finishActiveAssistant()
          addMessage(
            createMessage({
              id: `approval-required-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`,
              text: approvalPrompt.text,
              className: 'prompt',
              role: 'assistant',
              kind: 'text',
              status: 'confirmation',
              discussion: approvalPrompt.text,
              confirmationPrompt: approvalPrompt.text,
              confirmationOptions: approvalPrompt.confirmationOptions,
              timestamp: Date.now(),
            }),
          )
          return
        }
        const stageBlockedPrompt = parseStageToolBlockedResult(parsed.result)
        if (stageBlockedPrompt) {
          finishActiveAssistant()
          addMessage(
            createMessage({
              id: `stage-tool-blocked-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`,
              text: stageBlockedPrompt.text,
              className: 'prompt',
              role: 'assistant',
              kind: 'text',
              status: 'confirmation',
              discussion: stageBlockedPrompt.text,
              confirmationPrompt: stageBlockedPrompt.text,
              confirmationOptions: stageBlockedPrompt.confirmationOptions,
              timestamp: Date.now(),
            }),
          )
          return
        }

        const toolFailure = parseToolFailureResult(parsed.result)
        if (toolFailure) {
          appendDraftStateUpdate(`Tool error: ${toolFailure}`)
        }
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
  }, [
    addMessage,
    appendDraftCommand,
    appendDraftStateUpdate,
    clearReconnectTimer,
    ensureActiveAssistant,
    finishActiveAssistant,
    flushBufferedTokens,
    projectId,
    scheduleTokenFlush,
    token,
  ])

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
      clearActiveAssistantMetadata()
      wsRef.current?.close()
      wsRef.current = null
    }
  }, [clearActiveAssistantMetadata, clearReconnectTimer, connect])

  useEffect(() => {
    let canceled = false

    const cachedTimeline = projectId ? loadProjectTimeline(projectId) : []
    setMessages(cachedTimeline)
    lastEmittedMessageRef.current = ''
    tokenBufferRef.current = []
    activeAssistantIDRef.current = null
    clearActiveAssistantMetadata()
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
        const historyMessages = items.map(toHistorySessionMessage).filter((item): item is SessionMessage => item !== null)
        setMessages((prev) => mergeHistoryMessages(historyMessages, prev))
      } catch {
        // Keep any local in-flight/optimistic chat messages if the history request fails.
      }
    })()

    return () => {
      canceled = true
    }
  }, [clearActiveAssistantMetadata, projectId])

  useEffect(() => {
    if (!projectId) {
      return
    }
    saveProjectTimeline(projectId, messages)
  }, [messages, projectId])

  useEffect(() => {
    if (!projectId || messages.length === 0) {
      return
    }
    const lastMessage = messages[messages.length - 1]
    const fingerprint = `${projectId}:${lastMessage.id ?? ''}:${lastMessage.timestamp}:${lastMessage.text.slice(0, 80)}`
    if (lastEmittedMessageRef.current === fingerprint) {
      return
    }
    lastEmittedMessageRef.current = fingerprint
    orchestratorEventBus.emit(sessionMessageToEvent(projectId, lastMessage))
  }, [messages, projectId])

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
