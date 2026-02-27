import { useCallback, useEffect, useMemo, useState } from 'react'
import { useOrchestratorWS } from '../hooks/useOrchestratorWS'
import {
  getOrchestratorReport,
  listOrchestratorExceptions,
  resolveOrchestratorException,
  listDemandPoolItems,
  createDemandPoolItem,
  updateDemandPoolItem,
  deleteDemandPoolItem,
  promoteDemandPoolItem,
} from '../api/client'
import type {
  Project, Task, Session,
  OrchestratorProgressReport, OrchestratorExceptionItem,
  OrchestratorExceptionListResponse, DemandPoolItem,
} from '../api/types'
import type { SessionMessage, MessageTaskLink } from './ChatMessage'
import ChatPanel from './ChatPanel'
import StagePipeline from './StagePipeline'
import { X } from './Lucide'

interface OrchestratorPanelProps {
  project: Project | null
  projectID: string
  tasks: Task[]
  sessions: Session[]
  open: boolean
  onClose: () => void
  onOpenTaskSession: (taskID: string) => void
  onOpenDemandPool: () => void
}

function readAsString(value: unknown): string {
  return typeof value === 'string' ? value : ''
}

function readAsNumber(value: unknown): number {
  if (typeof value === 'number' && Number.isFinite(value)) {
    return value
  }
  return 0
}

function readAsStringArray(value: unknown): string[] {
  if (!Array.isArray(value)) {
    return []
  }
  return value.filter((item): item is string => typeof item === 'string')
}

function buildReportSummaryText(report: OrchestratorProgressReport): string {
  const phase = readAsString(report.phase) || 'unknown'
  const queueDepth = readAsNumber(report.queue_depth)
  const activeSessions = readAsNumber(report.active_sessions)
  const pendingTasks = readAsNumber(report.pending_tasks)
  const completedTasks = readAsNumber(report.completed_tasks)
  const reviewState = readAsString(report.review_state) || 'not_started'
  const openReviewIssues = readAsNumber(report.open_review_issues_total)
  const blockers = readAsStringArray(report.blockers)

  const lines = [
    `phase=${phase}`,
    `queue=${queueDepth}`,
    `sessions_active=${activeSessions}`,
    `tasks_pending=${pendingTasks}`,
    `tasks_done=${completedTasks}`,
    `review=${reviewState}`,
    `open_review_issues=${openReviewIssues}`,
  ]
  if (blockers.length > 0) {
    lines.push(`blockers=${blockers.join('; ')}`)
  }
  return lines.join('\n')
}

export default function OrchestratorPanel({
  project,
  projectID,
  tasks,
  sessions,
  open,
  onClose,
  onOpenTaskSession,
  onOpenDemandPool,
}: OrchestratorPanelProps) {
  const [pipelineCollapsed, setPipelineCollapsed] = useState(false)
  const [demandOpen, setDemandOpen] = useState(false)
  const [exceptionsOpen, setExceptionsOpen] = useState(false)

  // Demand pool items
  const [demandItems, setDemandItems] = useState<DemandPoolItem[]>([])
  const [demandFormOpen, setDemandFormOpen] = useState(false)
  const [demandTitle, setDemandTitle] = useState('')
  const [demandDesc, setDemandDesc] = useState('')
  const [demandPriority, setDemandPriority] = useState(3)
  const [editingItemID, setEditingItemID] = useState<string | null>(null)

  // Exceptions
  const [exceptions, setExceptions] = useState<OrchestratorExceptionItem[]>([])
  const [exceptionCounts, setExceptionCounts] = useState({ total: 0, open: 0, resolved: 0 })

  // Progress
  const [progressReport, setProgressReport] = useState<OrchestratorProgressReport | null>(null)
  const [reportUpdatedAt, setReportUpdatedAt] = useState<number | null>(null)
  const [reportLoading, setReportLoading] = useState(false)

  // Orchestrator WS
  const orchestrator = useOrchestratorWS(projectID)

  // Reset state when projectID changes
  useEffect(() => {
    setProgressReport(null)
    setReportUpdatedAt(null)
    setReportLoading(false)
    setDemandItems([])
    setExceptions([])
    setExceptionCounts({ total: 0, open: 0, resolved: 0 })
  }, [projectID])

  // Fetch demand pool items on mount and when projectID changes
  useEffect(() => {
    if (!projectID) {
      setDemandItems([])
      return
    }
    let canceled = false
    void (async () => {
      try {
        const items = await listDemandPoolItems<DemandPoolItem[]>(projectID)
        if (!canceled) {
          setDemandItems(items)
        }
      } catch {
        // Silently ignore -- demand pool may not be available
      }
    })()
    return () => {
      canceled = true
    }
  }, [projectID])

  // Demand pool CRUD helpers
  const refreshDemandItems = useCallback(async () => {
    if (!projectID) return
    try {
      const items = await listDemandPoolItems<DemandPoolItem[]>(projectID)
      setDemandItems(items)
    } catch { /* ignore */ }
  }, [projectID])

  const saveDemandItem = useCallback(async () => {
    if (!demandTitle.trim()) return
    try {
      if (editingItemID) {
        await updateDemandPoolItem(editingItemID, {
          title: demandTitle.trim(),
          description: demandDesc.trim(),
          priority: demandPriority,
        })
      } else {
        await createDemandPoolItem(projectID, {
          title: demandTitle.trim(),
          description: demandDesc.trim(),
          priority: demandPriority,
        })
      }
      setDemandFormOpen(false)
      setDemandTitle('')
      setDemandDesc('')
      setDemandPriority(3)
      setEditingItemID(null)
      await refreshDemandItems()
    } catch { /* ignore */ }
  }, [projectID, demandTitle, demandDesc, demandPriority, editingItemID, refreshDemandItems])

  const removeDemandItem = useCallback(async (itemID: string) => {
    try {
      await deleteDemandPoolItem(itemID)
      await refreshDemandItems()
    } catch { /* ignore */ }
  }, [refreshDemandItems])

  const promoteDemand = useCallback(async (itemID: string) => {
    try {
      await promoteDemandPoolItem(itemID)
      await refreshDemandItems()
    } catch { /* ignore */ }
  }, [refreshDemandItems])

  const startEditDemand = useCallback((item: DemandPoolItem) => {
    setEditingItemID(item.id)
    setDemandTitle(item.title)
    setDemandDesc(item.description)
    setDemandPriority(item.priority)
    setDemandFormOpen(true)
  }, [])

  const startNewDemand = useCallback(() => {
    setEditingItemID(null)
    setDemandTitle('')
    setDemandDesc('')
    setDemandPriority(3)
    setDemandFormOpen(true)
  }, [])

  // Fetch exceptions on mount and when projectID changes
  const refreshExceptions = useCallback(async () => {
    if (!projectID) {
      setExceptions([])
      setExceptionCounts({ total: 0, open: 0, resolved: 0 })
      return
    }
    try {
      const response = await listOrchestratorExceptions<OrchestratorExceptionListResponse>(projectID, 'open')
      setExceptions(response.items ?? [])
      setExceptionCounts(response.counts ?? { total: 0, open: 0, resolved: 0 })
    } catch {
      // Silently ignore
    }
  }, [projectID])

  useEffect(() => {
    void refreshExceptions()
  }, [refreshExceptions])

  // Refresh exceptions when orchestrator emits tool_result or done
  useEffect(() => {
    if (
      orchestrator.lastEvent &&
      (orchestrator.lastEvent.type === 'tool_result' || orchestrator.lastEvent.type === 'done')
    ) {
      void refreshExceptions()
    }
  }, [orchestrator.lastEvent, refreshExceptions])

  // Resolve exception
  const resolveException = useCallback(
    async (exceptionID: string) => {
      if (!projectID || !exceptionID) {
        return
      }
      try {
        await resolveOrchestratorException(projectID, exceptionID, 'resolved')
        await refreshExceptions()
      } catch {
        // Silently ignore
      }
    },
    [projectID, refreshExceptions],
  )

  // Request progress report
  const requestProgressReport = useCallback(async () => {
    if (!projectID) {
      return
    }
    setReportLoading(true)
    try {
      const report = await getOrchestratorReport<OrchestratorProgressReport>(projectID)
      setProgressReport(report)
      setReportUpdatedAt(Date.now())
    } catch {
      // Silently ignore
    } finally {
      setReportLoading(false)
    }
  }, [projectID])

  // Derive currentPhase from progressReport
  const currentPhase = useMemo(() => {
    return readAsString(progressReport?.phase) || 'plan'
  }, [progressReport])

  // Derive taskLinks from tasks array
  const taskLinks = useMemo<MessageTaskLink[]>(() => {
    const links: MessageTaskLink[] = []
    for (const task of tasks) {
      links.push({ id: task.id, label: task.id })
      links.push({ id: task.id, label: task.title })
    }
    return links
  }, [tasks])

  // Derive chatMessages from orchestrator.messages + optional progress report message
  const chatMessages = useMemo<SessionMessage[]>(() => {
    if (!progressReport || !reportUpdatedAt) {
      return orchestrator.messages
    }
    return [
      ...orchestrator.messages,
      {
        id: `progress-report-${reportUpdatedAt}`,
        text: `Progress report (${new Date(reportUpdatedAt).toLocaleTimeString()}):\n${buildReportSummaryText(progressReport)}`,
        className: 'prompt',
        role: 'system' as const,
        kind: 'text' as const,
        timestamp: reportUpdatedAt,
      },
    ]
  }, [orchestrator.messages, progressReport, reportUpdatedAt])

  if (!open) {
    return null
  }

  return (
    <aside className="orchestrator-panel">
      {/* Header */}
      <div className="orchestrator-panel-header">
        <span>Orchestrator</span>
        <button className="btn btn-ghost btn-icon" onClick={onClose} type="button">
          <X size={14} />
        </button>
      </div>

      {/* Stage Pipeline */}
      <StagePipeline
        currentPhase={currentPhase}
        sessions={sessions}
        collapsed={pipelineCollapsed}
        onToggle={() => setPipelineCollapsed((prev) => !prev)}
      />

      {/* Chat Area */}
      <ChatPanel
        messages={chatMessages}
        taskLinks={taskLinks}
        isStreaming={orchestrator.isStreaming}
        connectionStatus={orchestrator.connectionStatus}
        onSend={orchestrator.send}
        onTaskClick={onOpenTaskSession}
        onReportProgress={requestProgressReport}
        isFetchingReport={reportLoading}
      />

      {/* Expandable sections */}
      <div className="orchestrator-sections">
        {/* Demand Pool */}
        <div className="orchestrator-section-header" onClick={() => setDemandOpen((prev) => !prev)}>
          <span>{demandOpen ? '\u25be' : '\u25b8'} Demand Pool ({demandItems.length})</span>
          {demandOpen && (
            <button
              className="btn btn-ghost"
              onClick={(e) => { e.stopPropagation(); startNewDemand() }}
              style={{ fontSize: '11px', marginLeft: 'auto' }}
              type="button"
            >
              + Add
            </button>
          )}
        </div>
        {demandOpen && (
          <div className="orchestrator-section-content">
            {demandFormOpen && (
              <div className="demand-form" style={{ display: 'flex', flexDirection: 'column', gap: '6px', marginBottom: '8px', padding: '8px', background: 'var(--bg-surface-hover)', borderRadius: '6px' }}>
                <input
                  value={demandTitle}
                  onChange={(e) => setDemandTitle(e.target.value)}
                  placeholder="Title"
                  style={{ fontSize: '12px' }}
                />
                <textarea
                  value={demandDesc}
                  onChange={(e) => setDemandDesc(e.target.value)}
                  placeholder="Description (optional)"
                  rows={2}
                  style={{ fontSize: '12px' }}
                />
                <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
                  <label style={{ fontSize: '11px', color: 'var(--text-secondary)' }}>Priority</label>
                  <select
                    value={demandPriority}
                    onChange={(e) => setDemandPriority(Number(e.target.value))}
                    style={{ fontSize: '12px', flex: 1 }}
                  >
                    <option value={1}>1 — Critical</option>
                    <option value={2}>2 — High</option>
                    <option value={3}>3 — Medium</option>
                    <option value={4}>4 — Low</option>
                    <option value={5}>5 — Nice-to-have</option>
                  </select>
                </div>
                <div style={{ display: 'flex', gap: '4px' }}>
                  <button className="btn btn-primary" onClick={() => void saveDemandItem()} style={{ fontSize: '11px' }} type="button">
                    {editingItemID ? 'Update' : 'Create'}
                  </button>
                  <button className="btn btn-ghost" onClick={() => { setDemandFormOpen(false); setEditingItemID(null) }} style={{ fontSize: '11px' }} type="button">
                    Cancel
                  </button>
                </div>
              </div>
            )}
            {demandItems.map((item) => (
              <div className="demand-item" key={item.id}>
                <span className={`demand-status-badge ${item.status}`}>{item.status}</span>
                <span style={{ flex: 1, fontSize: '12px' }}>{item.title || item.description?.slice(0, 60)}</span>
                <button className="btn btn-ghost" onClick={() => startEditDemand(item)} style={{ fontSize: '10px', padding: '2px 4px' }} type="button">Edit</button>
                <button className="btn btn-ghost" onClick={() => void promoteDemand(item.id)} style={{ fontSize: '10px', padding: '2px 4px' }} type="button">Promote</button>
                <button className="btn btn-ghost" onClick={() => void removeDemandItem(item.id)} style={{ fontSize: '10px', padding: '2px 4px', color: 'var(--status-red)' }} type="button">Del</button>
              </div>
            ))}
            {demandItems.length === 0 && !demandFormOpen && (
              <p style={{ color: 'var(--text-tertiary)', fontSize: '12px' }}>No items</p>
            )}
          </div>
        )}

        {/* Exceptions */}
        <div className="orchestrator-section-header" onClick={() => setExceptionsOpen((prev) => !prev)}>
          <span>{exceptionsOpen ? '\u25be' : '\u25b8'} Exceptions ({exceptionCounts.open})</span>
        </div>
        {exceptionsOpen && (
          <div className="orchestrator-section-content">
            {exceptions.map((item) => (
              <div className="exception-item" key={item.id}>
                <div className="exception-meta">
                  <div className="exception-category">{item.category}</div>
                  <div className="exception-message">{item.message}</div>
                </div>
                <button
                  className="btn btn-ghost"
                  onClick={() => void resolveException(item.id)}
                  style={{ fontSize: '11px' }}
                  type="button"
                >
                  Resolve
                </button>
              </div>
            ))}
            {exceptions.length === 0 && (
              <p style={{ color: 'var(--text-tertiary)', fontSize: '12px' }}>No open exceptions</p>
            )}
          </div>
        )}
      </div>
    </aside>
  )
}
