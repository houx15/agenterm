import type { SessionMessage } from '../components/ChatMessage'

const STORAGE_KEY = 'agenterm:orchestrator:timeline:v1'
const MAX_MESSAGES_PER_PROJECT = 300

type StoredTimelineState = {
  version: 1
  projects: Record<string, SessionMessage[]>
}

function createEmptyState(): StoredTimelineState {
  return {
    version: 1,
    projects: {},
  }
}

function parseState(raw: string | null): StoredTimelineState {
  if (!raw) {
    return createEmptyState()
  }
  try {
    const parsed = JSON.parse(raw) as StoredTimelineState
    if (!parsed || typeof parsed !== 'object' || parsed.version !== 1 || typeof parsed.projects !== 'object' || parsed.projects == null) {
      return createEmptyState()
    }
    return parsed
  } catch {
    return createEmptyState()
  }
}

function readState(): StoredTimelineState {
  if (typeof window === 'undefined') {
    return createEmptyState()
  }
  return parseState(window.localStorage.getItem(STORAGE_KEY))
}

function writeState(state: StoredTimelineState): void {
  if (typeof window === 'undefined') {
    return
  }
  window.localStorage.setItem(STORAGE_KEY, JSON.stringify(state))
}

function normalizeMessage(message: SessionMessage): SessionMessage {
  return {
    id: message.id,
    text: message.text,
    className: message.className,
    timestamp: message.timestamp,
    isUser: message.isUser,
    role: message.role,
    kind: message.kind,
    status: message.status,
    confirmationOptions: message.confirmationOptions,
    discussion: message.discussion,
    commands: message.commands,
    stateUpdates: message.stateUpdates,
    confirmationPrompt: message.confirmationPrompt,
  }
}

export function loadProjectTimeline(projectId: string): SessionMessage[] {
  const key = projectId.trim()
  if (!key) {
    return []
  }
  const state = readState()
  const items = state.projects[key]
  if (!Array.isArray(items)) {
    return []
  }
  return items
    .filter((item) => item && typeof item === 'object')
    .map((item) => normalizeMessage(item))
    .slice(-MAX_MESSAGES_PER_PROJECT)
}

export function saveProjectTimeline(projectId: string, messages: SessionMessage[]): void {
  const key = projectId.trim()
  if (!key) {
    return
  }
  const state = readState()
  const persisted = messages
    .filter((message) => {
      if ((message.text ?? '').trim()) {
        return true
      }
      if ((message.discussion ?? '').trim()) {
        return true
      }
      if ((message.commands ?? []).length > 0) {
        return true
      }
      if ((message.stateUpdates ?? []).length > 0) {
        return true
      }
      return false
    })
    .map(normalizeMessage)
    .slice(-MAX_MESSAGES_PER_PROJECT)
  state.projects[key] = persisted
  writeState(state)
}
