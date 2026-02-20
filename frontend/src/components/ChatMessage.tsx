import type { ActionMessage } from '../api/types'
import type { ReactNode } from 'react'
import { Hammer, MessageSquare, ShieldCheck } from 'lucide-react'

export interface MessageTaskLink {
  id: string
  label: string
}

export interface MessageActionOption {
  label: string
  value: string
}

type ChatRole = 'user' | 'assistant' | 'tool' | 'system' | 'error'
type ChatKind = 'text' | 'tool_call' | 'tool_result' | 'error'
type ChatStatus = 'discussion' | 'command' | 'confirmation'

export interface SessionMessage {
  id?: string
  text: string
  className: string
  actions?: ActionMessage[]
  timestamp: number
  isUser?: boolean
  role?: ChatRole
  kind?: ChatKind
  status?: ChatStatus
  taskLinks?: MessageTaskLink[]
  confirmationOptions?: MessageActionOption[]
}

interface ChatMessageProps {
  message: SessionMessage
  variant?: 'terminal' | 'pm'
  onTaskClick?: (taskID: string) => void
  onActionClick?: (value: string, label: string, messageID?: string) => void
}

export default function ChatMessage({ message, variant = 'terminal', onTaskClick, onActionClick }: ChatMessageProps) {
  if (variant === 'pm') {
    const roleClass = message.isUser ? 'user' : (message.role ?? 'assistant')
    const kindClass = message.kind ? `kind-${message.kind}` : 'kind-text'
    const statusClass = message.status ?? 'discussion'
    return (
      <div className={`pm-chat-message ${roleClass} ${kindClass} ${statusClass}`.trim()}>
        <div className="pm-chat-status">
          {message.status === 'command' && <Hammer size={12} />}
          {message.status === 'confirmation' && <ShieldCheck size={12} />}
          {(!message.status || message.status === 'discussion') && <MessageSquare size={12} />}
          <span>{message.status ?? 'discussion'}</span>
        </div>
        <div className="pm-chat-bubble">{renderPMBubbleContent(message, onTaskClick)}</div>
        {message.confirmationOptions && message.confirmationOptions.length > 0 && (
          <div className="pm-confirm-row">
            {message.confirmationOptions.map((option) => (
              <button
                key={`${option.label}-${option.value}`}
                className="action-btn"
                onClick={() => onActionClick?.(option.value, option.label, message.id)}
                type="button"
              >
                {option.label}
              </button>
            ))}
          </div>
        )}
      </div>
    )
  }

  const cssClass = message.isUser ? 'input' : normalizeClass(message.className)
  return <div className={`term-line ${cssClass}`}>{message.text}</div>
}

function normalizeClass(value: string): string {
  const lower = value.toLowerCase()
  if (lower.includes('error')) return 'error'
  if (lower.includes('prompt')) return 'prompt'
  if (lower.includes('code')) return 'code'
  if (lower.includes('input')) return 'input'
  return 'output'
}

function escapeRegex(value: string): string {
  return value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
}

function renderTextWithTaskLinks(text: string, taskLinks?: MessageTaskLink[], onTaskClick?: (taskID: string) => void): ReactNode {
  if (!taskLinks || taskLinks.length === 0 || !text.trim() || !onTaskClick) {
    return text
  }

  const sorted = [...taskLinks]
    .filter((link) => link.label.trim())
    .sort((left, right) => right.label.length - left.label.length)

  if (sorted.length === 0) {
    return text
  }

  const pattern = new RegExp(`(${sorted.map((item) => escapeRegex(item.label)).join('|')})`, 'gi')
  const parts = text.split(pattern)

  return parts.map((part, idx) => {
    const found = sorted.find((item) => item.label.toLowerCase() === part.toLowerCase())
    if (!found) {
      return <span key={`txt-${idx}`}>{part}</span>
    }

    return (
      <button key={`task-${found.id}-${idx}`} className="pm-inline-task-link" onClick={() => onTaskClick(found.id)} type="button">
        {part}
      </button>
    )
  })
}

function tryParseJSON(text: string): unknown | null {
  try {
    return JSON.parse(text)
  } catch {
    return null
  }
}

function parseToolCallText(text: string): { name: string; args: unknown | null } | null {
  const trimmed = text.trim()
  if (!trimmed.startsWith('[') || !trimmed.endsWith(']')) {
    return null
  }
  const body = trimmed.slice(1, -1).trim()
  if (!body || body.startsWith('✅')) {
    return null
  }
  const firstSpace = body.indexOf(' ')
  if (firstSpace < 0) {
    return { name: body, args: null }
  }
  const name = body.slice(0, firstSpace).trim()
  const argsText = body.slice(firstSpace + 1).trim()
  return { name, args: tryParseJSON(argsText) ?? argsText }
}

function parseToolResultText(text: string): { ok: boolean; payload: unknown | null; rawPayload: string } | null {
  const trimmed = text.trim()
  if (!trimmed.startsWith('[✅') || !trimmed.endsWith(']')) {
    return null
  }
  const payloadText = trimmed.slice(2, -1).replace(/^✅/, '').trim()
  if (!payloadText) {
    return { ok: true, payload: null, rawPayload: '' }
  }
  return {
    ok: true,
    payload: tryParseJSON(payloadText),
    rawPayload: payloadText,
  }
}

function renderDataBlock(value: unknown): ReactNode {
  if (value == null) {
    return null
  }
  if (typeof value === 'string') {
    return <pre className="pm-tool-json">{value}</pre>
  }
  return <pre className="pm-tool-json">{JSON.stringify(value, null, 2)}</pre>
}

function renderPMBubbleContent(message: SessionMessage, onTaskClick?: (taskID: string) => void): ReactNode {
  if (message.kind === 'tool_call') {
    const parsed = parseToolCallText(message.text)
    if (parsed) {
      return (
        <div className="pm-tool-card">
          <div className="pm-tool-title">Tool: {parsed.name}</div>
          {parsed.args != null && (
            <div className="pm-tool-section">
              <small>Arguments</small>
              {renderDataBlock(parsed.args)}
            </div>
          )}
        </div>
      )
    }
  }

  if (message.kind === 'tool_result') {
    const parsed = parseToolResultText(message.text)
    if (parsed) {
      const payloadObject = parsed.payload && typeof parsed.payload === 'object' && !Array.isArray(parsed.payload) ? (parsed.payload as Record<string, unknown>) : null
      const error = payloadObject && typeof payloadObject.error === 'string' ? payloadObject.error : ''
      return (
        <div className="pm-tool-card">
          <div className={`pm-tool-title ${error ? 'error' : 'ok'}`.trim()}>{error ? `Result: ${error}` : 'Result: success'}</div>
          {(parsed.payload != null || parsed.rawPayload) && (
            <div className="pm-tool-section">
              <small>Output</small>
              {renderDataBlock(parsed.payload ?? parsed.rawPayload)}
            </div>
          )}
        </div>
      )
    }
  }

  return renderTextWithTaskLinks(message.text, message.taskLinks, onTaskClick)
}
