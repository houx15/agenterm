import type { ActionMessage } from '../api/types'

export interface SessionMessage {
  text: string
  className: string
  actions?: ActionMessage[]
  timestamp: number
  isUser?: boolean
}

interface ChatMessageProps {
  message: SessionMessage
}

export default function ChatMessage({ message }: ChatMessageProps) {
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
