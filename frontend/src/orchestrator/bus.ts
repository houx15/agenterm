import type { OrchestratorTimelineEvent } from './schema'

type TimelineListener = (event: OrchestratorTimelineEvent) => void

class OrchestratorProjectEventBus {
  private listeners: Map<string, Set<TimelineListener>> = new Map()

  subscribe(projectId: string, listener: TimelineListener): () => void {
    const key = projectId.trim()
    if (!key) {
      return () => {}
    }
    const projectListeners = this.listeners.get(key) ?? new Set<TimelineListener>()
    projectListeners.add(listener)
    this.listeners.set(key, projectListeners)

    return () => {
      const current = this.listeners.get(key)
      if (!current) {
        return
      }
      current.delete(listener)
      if (current.size === 0) {
        this.listeners.delete(key)
      }
    }
  }

  emit(event: OrchestratorTimelineEvent): void {
    const projectListeners = this.listeners.get(event.projectId)
    if (!projectListeners || projectListeners.size === 0) {
      return
    }
    for (const listener of projectListeners) {
      listener(event)
    }
  }
}

export const orchestratorEventBus = new OrchestratorProjectEventBus()
