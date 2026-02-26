import type { SessionMessage } from '../components/ChatMessage'

export type OrchestratorTimelineEventType =
  | 'discussion'
  | 'command'
  | 'state_update'
  | 'confirmation_required'
  | 'user_message'
  | 'error'

export interface OrchestratorTimelineEvent {
  id: string
  projectId: string
  type: OrchestratorTimelineEventType
  timestamp: number
  text: string
  status?: SessionMessage['status']
}

function classifyMessageType(message: SessionMessage): OrchestratorTimelineEventType {
  if (message.role === 'user' || message.isUser) {
    return 'user_message'
  }
  if (message.role === 'error' || message.kind === 'error') {
    return 'error'
  }
  if (message.status === 'confirmation') {
    return 'confirmation_required'
  }
  if (message.status === 'command') {
    return 'command'
  }
  if ((message.stateUpdates ?? []).length > 0) {
    return 'state_update'
  }
  return 'discussion'
}

function safeMessageID(message: SessionMessage): string {
  if (message.id && message.id.trim()) {
    return message.id
  }
  const seed = `${message.timestamp}:${message.text.slice(0, 48)}`
  return `msg-${Math.abs(hash(seed))}`
}

function hash(value: string): number {
  let out = 0
  for (let i = 0; i < value.length; i += 1) {
    out = (out << 5) - out + value.charCodeAt(i)
    out |= 0
  }
  return out
}

export function sessionMessageToEvent(projectId: string, message: SessionMessage): OrchestratorTimelineEvent {
  return {
    id: `${projectId}:${safeMessageID(message)}`,
    projectId,
    type: classifyMessageType(message),
    timestamp: message.timestamp,
    text: message.text,
    status: message.status,
  }
}
