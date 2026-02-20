import { useCallback, useEffect, useMemo, useState } from 'react'
import { ArrowDown, ArrowUp, FileText, Filter, Lightbulb, Pencil, RefreshCw, Rocket, Sparkles, Trash2 } from 'lucide-react'
import {
  chatDemandOrchestrator,
  createDemandPoolItem,
  deleteDemandPoolItem,
  getDemandOrchestratorReport,
  listDemandPoolItems,
  listDemandOrchestratorHistory,
  listProjects,
  promoteDemandPoolItem,
  reprioritizeDemandPool,
  updateDemandPoolItem,
} from '../api/client'
import type { DemandPoolItem, DemandPoolStatus, OrchestratorHistoryMessage, Project } from '../api/types'
import Modal from '../components/Modal'

interface DemandPoolProps {
  projectID?: string
  projectName?: string
  embedded?: boolean
  readOnly?: boolean
}

type EditableDemandFields = Pick<DemandPoolItem, 'title' | 'description' | 'status' | 'priority' | 'impact' | 'effort' | 'risk' | 'urgency' | 'notes'>

const STATUS_OPTIONS: DemandPoolStatus[] = ['captured', 'triaged', 'shortlisted', 'scheduled', 'done', 'rejected']

const EMPTY_EDITABLE: EditableDemandFields = {
  title: '',
  description: '',
  status: 'captured',
  priority: 0,
  impact: 3,
  effort: 3,
  risk: 3,
  urgency: 3,
  notes: '',
}

function parseTags(value: string): string[] {
  return value
    .split(',')
    .map((tag) => tag.trim())
    .filter(Boolean)
}

function tagsToText(tags: string[]): string {
  return tags.join(', ')
}

export default function DemandPool({ projectID: pinnedProjectID = '', projectName, embedded = false, readOnly = false }: DemandPoolProps) {
  const [projects, setProjects] = useState<Project[]>([])
  const [projectID, setProjectID] = useState(pinnedProjectID)
  const [items, setItems] = useState<DemandPoolItem[]>([])
  const [statusFilter, setStatusFilter] = useState('')
  const [queryFilter, setQueryFilter] = useState('')
  const [quickTitle, setQuickTitle] = useState('')
  const [quickTags, setQuickTags] = useState('')
  const [message, setMessage] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const [busyItemID, setBusyItemID] = useState('')
  const [modalOpen, setModalOpen] = useState(false)
  const [editTarget, setEditTarget] = useState<DemandPoolItem | null>(null)
  const [draft, setDraft] = useState<EditableDemandFields>(EMPTY_EDITABLE)
  const [draftTags, setDraftTags] = useState('')
  const [demandHistory, setDemandHistory] = useState<OrchestratorHistoryMessage[]>([])
  const [demandInput, setDemandInput] = useState('')
  const [demandReport, setDemandReport] = useState<Record<string, unknown> | null>(null)
  const [localPriorities, setLocalPriorities] = useState<Record<string, number>>({})
  const [digestSummary, setDigestSummary] = useState('')

  const selectedProject = useMemo(() => projects.find((project) => project.id === projectID) ?? null, [projects, projectID])
  const resolvedProjectName = projectName || selectedProject?.name || 'selected project'

  const loadDemandItems = useCallback(async () => {
    if (!projectID) {
      setItems([])
      return
    }
    setLoading(true)
    setError('')
    try {
      const data = await listDemandPoolItems<DemandPoolItem[]>(projectID, {
        status: statusFilter || undefined,
        q: queryFilter.trim() || undefined,
      })
      setItems(data)
      setLocalPriorities(Object.fromEntries(data.map((item) => [item.id, item.priority])))
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load demand pool items')
    } finally {
      setLoading(false)
    }
  }, [projectID, queryFilter, statusFilter])

  const sortedItems = useMemo(() => {
    const list = [...items]
    list.sort((a, b) => {
      const pa = localPriorities[a.id] ?? a.priority
      const pb = localPriorities[b.id] ?? b.priority
      if (pa !== pb) {
        return pb - pa
      }
      return a.created_at < b.created_at ? 1 : -1
    })
    return list
  }, [items, localPriorities])

  const hasPriorityChanges = useMemo(() => {
    return items.some((item) => (localPriorities[item.id] ?? item.priority) !== item.priority)
  }, [items, localPriorities])

  const loadDemandLaneContext = useCallback(async () => {
    if (!projectID) {
      setDemandHistory([])
      setDemandReport(null)
      return
    }
    try {
      const [history, report] = await Promise.all([
        listDemandOrchestratorHistory<OrchestratorHistoryMessage[]>(projectID, 30),
        getDemandOrchestratorReport<Record<string, unknown>>(projectID),
      ])
      setDemandHistory(history)
      setDemandReport(report)
    } catch {
      // Demand lane context is supplemental; keep page usable even when unavailable.
    }
  }, [projectID])

  useEffect(() => {
    if (pinnedProjectID) {
      setProjectID(pinnedProjectID)
    }
  }, [pinnedProjectID])

  useEffect(() => {
    if (pinnedProjectID) {
      setProjects([])
      return
    }
    let cancelled = false
    const load = async () => {
      try {
        const data = await listProjects<Project[]>()
        if (cancelled) {
          return
        }
        setProjects(data)
        if (data.length > 0) {
          setProjectID((current) => {
            if (current && data.some((project) => project.id === current)) {
              return current
            }
            return data[0].id
          })
        } else {
          setProjectID('')
        }
      } catch (err) {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : 'Failed to load projects')
        }
      }
    }
    void load()
    return () => {
      cancelled = true
    }
  }, [pinnedProjectID])

  useEffect(() => {
    void loadDemandItems()
  }, [loadDemandItems])

  useEffect(() => {
    void loadDemandLaneContext()
  }, [loadDemandLaneContext])

  function openCreateModal() {
    setEditTarget(null)
    setDraft(EMPTY_EDITABLE)
    setDraftTags('')
    setModalOpen(true)
    setMessage('')
    setError('')
  }

  function openEditModal(item: DemandPoolItem) {
    setEditTarget(item)
    setDraft({
      title: item.title,
      description: item.description,
      status: (STATUS_OPTIONS.find((status) => status === item.status) ?? 'captured') as DemandPoolStatus,
      priority: item.priority,
      impact: item.impact,
      effort: item.effort,
      risk: item.risk,
      urgency: item.urgency,
      notes: item.notes,
    })
    setDraftTags(tagsToText(item.tags))
    setModalOpen(true)
    setMessage('')
    setError('')
  }

  async function submitQuickAdd() {
    if (readOnly) {
      return
    }
    if (!projectID || !quickTitle.trim()) {
      return
    }
    setBusyItemID('quick-add')
    setError('')
    setMessage('')
    try {
      await createDemandPoolItem<DemandPoolItem>(projectID, {
        title: quickTitle.trim(),
        tags: parseTags(quickTags),
      })
      setQuickTitle('')
      setQuickTags('')
      setMessage('Demand captured.')
      await loadDemandItems()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create demand')
    } finally {
      setBusyItemID('')
    }
  }

  async function submitModal() {
    if (readOnly) {
      return
    }
    if (!projectID || !draft.title.trim()) {
      setError('Title is required.')
      return
    }
    setBusyItemID(editTarget?.id ?? 'modal-create')
    setError('')
    try {
      const payload = {
        ...draft,
        title: draft.title.trim(),
        description: draft.description.trim(),
        notes: draft.notes.trim(),
        tags: parseTags(draftTags),
      }
      if (editTarget) {
        await updateDemandPoolItem<DemandPoolItem>(editTarget.id, payload)
        setMessage('Demand updated.')
      } else {
        await createDemandPoolItem<DemandPoolItem>(projectID, payload)
        setMessage('Demand created.')
      }
      setEditTarget(null)
      setModalOpen(false)
      await loadDemandItems()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to save demand')
    } finally {
      setBusyItemID('')
    }
  }

  async function promoteItem(item: DemandPoolItem) {
    if (readOnly) {
      return
    }
    setBusyItemID(item.id)
    setError('')
    setMessage('')
    try {
      await promoteDemandPoolItem(item.id, {})
      setMessage(`Promoted "${item.title}" to task.`)
      await loadDemandItems()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to promote demand')
    } finally {
      setBusyItemID('')
    }
  }

  async function removeItem(item: DemandPoolItem) {
    if (readOnly) {
      return
    }
    setBusyItemID(item.id)
    setError('')
    setMessage('')
    try {
      await deleteDemandPoolItem(item.id)
      setMessage(`Deleted "${item.title}".`)
      await loadDemandItems()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to delete demand')
    } finally {
      setBusyItemID('')
    }
  }

  async function sendDemandChat() {
    if (readOnly) {
      return
    }
    if (!projectID || !demandInput.trim()) {
      return
    }
    const userText = demandInput.trim()
    setBusyItemID('demand-chat')
    setError('')
    setMessage('')
    setDemandInput('')
    try {
      const response = await chatDemandOrchestrator<{ response?: string } | undefined>({
        project_id: projectID,
        message: userText,
      })
      await loadDemandItems()
      await loadDemandLaneContext()
      if (response?.response) {
        setMessage('Demand assistant responded.')
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to send message to demand assistant')
    } finally {
      setBusyItemID('')
    }
  }

  function setItemPriority(itemID: string, value: number) {
    setLocalPriorities((prev) => ({ ...prev, [itemID]: value }))
  }

  function moveItemPriority(itemID: string, direction: 'up' | 'down') {
    const index = sortedItems.findIndex((item) => item.id === itemID)
    if (index < 0) {
      return
    }
    const swapIndex = direction === 'up' ? index - 1 : index + 1
    if (swapIndex < 0 || swapIndex >= sortedItems.length) {
      return
    }
    const current = sortedItems[index]
    const target = sortedItems[swapIndex]
    const currentPriority = localPriorities[current.id] ?? current.priority
    const targetPriority = localPriorities[target.id] ?? target.priority
    setLocalPriorities((prev) => ({
      ...prev,
      [current.id]: targetPriority,
      [target.id]: currentPriority,
    }))
  }

  async function saveReprioritization() {
    if (readOnly) {
      return
    }
    if (!projectID || !hasPriorityChanges) {
      return
    }
    setBusyItemID('reprioritize')
    setError('')
    setMessage('')
    try {
      const changed = items
        .map((item) => ({ id: item.id, priority: localPriorities[item.id] ?? item.priority, original: item.priority }))
        .filter((entry) => entry.priority !== entry.original)
        .map(({ id, priority }) => ({ id, priority }))
      if (changed.length === 0) {
        return
      }
      await reprioritizeDemandPool(projectID, { items: changed })
      setMessage(`Saved ${changed.length} reprioritized item(s).`)
      await loadDemandItems()
      await loadDemandLaneContext()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to reprioritize demand pool')
    } finally {
      setBusyItemID('')
    }
  }

  function resetLocalPriorities() {
    setLocalPriorities(Object.fromEntries(items.map((item) => [item.id, item.priority])))
  }

  async function rerankNow() {
    if (readOnly) {
      return
    }
    if (!projectID) {
      return
    }
    setBusyItemID('rerank')
    setError('')
    setMessage('')
    try {
      await chatDemandOrchestrator<{ response?: string }>({
        project_id: projectID,
        message:
          'Approved: reprioritize the demand pool now. Use reprioritize_demand_pool and then summarize your rationale in 5 bullets.',
      })
      setMessage('Demand assistant reprioritized the pool.')
      await loadDemandItems()
      await loadDemandLaneContext()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to run AI rerank')
    } finally {
      setBusyItemID('')
    }
  }

  async function generateDigest() {
    if (readOnly) {
      return
    }
    if (!projectID) {
      return
    }
    setBusyItemID('digest')
    setError('')
    setMessage('')
    try {
      const response = await chatDemandOrchestrator<{ response?: string }>({
        project_id: projectID,
        message:
          'Generate a concise backlog digest: top 5 demands, why now, what can wait, and one recommended next item to promote.',
      })
      setDigestSummary((response?.response || '').trim())
      await loadDemandLaneContext()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to generate digest')
    } finally {
      setBusyItemID('')
    }
  }

  return (
    <section className={`${embedded ? 'demand-page demand-page-embedded' : 'page-block demand-page'}`.trim()}>
      <div className="demand-header">
        <div>
          {embedded ? <h3>Demand Pool</h3> : <h2>Demand Pool</h2>}
          <p>Capture ideas and prioritize them separately from active development execution.</p>
          {readOnly && <p className="demand-readonly-hint">Read-only view. Add/edit demands in PM Chat.</p>}
        </div>
        {!readOnly && (
          <button className="primary-btn" onClick={openCreateModal} type="button">
            <Lightbulb size={16} />
            <span>Rich Add</span>
          </button>
        )}
      </div>

      <div className="dashboard-section demand-controls">
        {!pinnedProjectID && (
          <label>
            <span>Project</span>
            <select value={projectID} onChange={(event) => setProjectID(event.target.value)}>
              {projects.map((project) => (
                <option key={project.id} value={project.id}>
                  {project.name}
                </option>
              ))}
            </select>
          </label>
        )}
        <label>
          <span>Status</span>
          <select value={statusFilter} onChange={(event) => setStatusFilter(event.target.value)}>
            <option value="">All</option>
            {STATUS_OPTIONS.map((status) => (
              <option key={status} value={status}>
                {status}
              </option>
            ))}
          </select>
        </label>
        <label>
          <span>Search</span>
          <input placeholder="title/description/tags" value={queryFilter} onChange={(event) => setQueryFilter(event.target.value)} />
        </label>
        <button className="secondary-btn" onClick={() => void loadDemandItems()} type="button">
          <Filter size={16} />
          <span>Apply Filters</span>
        </button>
      </div>

      {!readOnly && (
        <div className="dashboard-section demand-quick-add">
          <label>
            <span>Quick Add</span>
            <input
              placeholder={projectID ? `Add an idea for ${resolvedProjectName}` : 'Select a project first'}
              value={quickTitle}
              onChange={(event) => setQuickTitle(event.target.value)}
            />
          </label>
          <label>
            <span>Tags</span>
            <input placeholder="ux, backend, infra" value={quickTags} onChange={(event) => setQuickTags(event.target.value)} />
          </label>
          <button className="primary-btn" disabled={!projectID || !quickTitle.trim() || busyItemID === 'quick-add'} onClick={() => void submitQuickAdd()} type="button">
            Capture
          </button>
        </div>
      )}

      {(message || error) && (
        <p className={`settings-message ${error ? 'text-error' : ''}`.trim()}>{error || message}</p>
      )}

      <div className="dashboard-section demand-list">
        {!readOnly && (
          <div className="demand-list-toolbar">
            <button className="secondary-btn" disabled={!hasPriorityChanges || busyItemID === 'reprioritize'} onClick={() => void saveReprioritization()} type="button">
              <Sparkles size={14} />
              <span>Save Priority Changes</span>
            </button>
            <button className="secondary-btn" disabled={!hasPriorityChanges} onClick={resetLocalPriorities} type="button">
              <RefreshCw size={14} />
              <span>Reset</span>
            </button>
            <button className="primary-btn" disabled={busyItemID === 'rerank'} onClick={() => void rerankNow()} type="button">
              <Sparkles size={14} />
              <span>AI Re-rank Now</span>
            </button>
            <button className="secondary-btn" disabled={busyItemID === 'digest'} onClick={() => void generateDigest()} type="button">
              <FileText size={14} />
              <span>Generate Digest</span>
            </button>
          </div>
        )}
        {loading ? (
          <p className="empty-text">Loading demand pool...</p>
        ) : sortedItems.length === 0 ? (
          <p className="empty-text">No demand items yet for this filter.</p>
        ) : (
          <div className="demand-table-wrap">
            <table className="demand-table">
              <thead>
                <tr>
                  <th>Title</th>
                  <th>Status</th>
                  <th>Priority</th>
                  <th>Impact/Effort</th>
                  <th>Tags</th>
                  <th>Actions</th>
                </tr>
              </thead>
              <tbody>
                {sortedItems.map((item, index) => (
                  <tr key={item.id}>
                    <td>
                      <strong>{item.title}</strong>
                      {item.description ? <p>{item.description}</p> : null}
                    </td>
                    <td>{item.status}</td>
                    <td>
                      {readOnly ? (
                        item.priority
                      ) : (
                        <div className="demand-priority-cell">
                          <button className="icon-only-btn secondary-btn" disabled={index === 0} onClick={() => moveItemPriority(item.id, 'up')} type="button">
                            <ArrowUp size={13} />
                          </button>
                          <button
                            className="icon-only-btn secondary-btn"
                            disabled={index === sortedItems.length - 1}
                            onClick={() => moveItemPriority(item.id, 'down')}
                            type="button"
                          >
                            <ArrowDown size={13} />
                          </button>
                          <input
                            type="number"
                            value={localPriorities[item.id] ?? item.priority}
                            onChange={(event) => setItemPriority(item.id, Number(event.target.value) || 0)}
                          />
                        </div>
                      )}
                    </td>
                    <td>
                      {item.impact}/{item.effort}
                    </td>
                    <td>{item.tags.join(', ') || '-'}</td>
                    <td>
                      {readOnly ? (
                        <span className="empty-text">View only</span>
                      ) : (
                        <div className="demand-row-actions">
                          <button className="secondary-btn" onClick={() => openEditModal(item)} type="button">
                            <Pencil size={14} />
                            <span>Edit</span>
                          </button>
                          <button
                            className="primary-btn"
                            disabled={busyItemID === item.id || item.status === 'scheduled' || item.status === 'done' || Boolean(item.selected_task_id)}
                            onClick={() => void promoteItem(item)}
                            type="button"
                          >
                            <Rocket size={14} />
                            <span>Promote</span>
                          </button>
                          <button className="action-btn danger" disabled={busyItemID === item.id} onClick={() => void removeItem(item)} type="button">
                            <Trash2 size={14} />
                          </button>
                        </div>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {digestSummary && (
        <div className="dashboard-section">
          <h3>Latest Digest</h3>
          <p className="demand-digest">{digestSummary}</p>
        </div>
      )}

      <div className="dashboard-section demand-assistant-section">
        <div className="demand-assistant-header">
          <h3>Demand Assistant</h3>
          <button className="secondary-btn" onClick={() => void loadDemandLaneContext()} type="button">
            Refresh
          </button>
        </div>
        {demandReport ? (
          <p className="demand-report-summary">
            total={String(demandReport.demand_items_total ?? 0)} | awaiting_triage={String(demandReport.awaiting_triage_total ?? 0)} |
            awaiting_scheduling={String(demandReport.awaiting_scheduling_total ?? 0)}
          </p>
        ) : (
          <p className="empty-text">Demand report unavailable.</p>
        )}
        <div className="demand-chat-history">
          {demandHistory.length === 0 ? (
            <p className="empty-text">No demand assistant history yet.</p>
          ) : (
            demandHistory.map((item) => (
              <article key={item.id} className={`demand-chat-row ${item.role === 'assistant' ? 'assistant' : 'user'}`.trim()}>
                <strong>{item.role === 'assistant' ? 'Assistant' : 'You'}</strong>
                <p>{item.content}</p>
              </article>
            ))
          )}
        </div>
        {!readOnly && (
          <div className="demand-chat-input">
            <textarea
              rows={3}
              placeholder="Ask for triage suggestions, reprioritization ideas, or demand-to-task promotion guidance."
              value={demandInput}
              onChange={(event) => setDemandInput(event.target.value)}
            />
            <button className="primary-btn" disabled={!projectID || !demandInput.trim() || busyItemID === 'demand-chat'} onClick={() => void sendDemandChat()} type="button">
              Send
            </button>
          </div>
        )}
      </div>

      <Modal
        open={modalOpen}
        title={editTarget ? 'Edit Demand Item' : 'Create Demand Item'}
        onClose={() => {
          setEditTarget(null)
          setModalOpen(false)
        }}
      >
        <form
          className="modal-form demand-modal-form"
          onSubmit={(event) => {
            event.preventDefault()
            void submitModal()
          }}
        >
          <label>
            <span>Title</span>
            <input value={draft.title} onChange={(event) => setDraft((prev) => ({ ...prev, title: event.target.value }))} />
          </label>
          <label>
            <span>Description</span>
            <textarea rows={4} value={draft.description} onChange={(event) => setDraft((prev) => ({ ...prev, description: event.target.value }))} />
          </label>
          <div className="demand-modal-grid">
            <label>
              <span>Status</span>
              <select value={draft.status} onChange={(event) => setDraft((prev) => ({ ...prev, status: event.target.value as DemandPoolStatus }))}>
                {STATUS_OPTIONS.map((status) => (
                  <option key={status} value={status}>
                    {status}
                  </option>
                ))}
              </select>
            </label>
            <label>
              <span>Priority</span>
              <input type="number" value={draft.priority} onChange={(event) => setDraft((prev) => ({ ...prev, priority: Number(event.target.value) || 0 }))} />
            </label>
            <label>
              <span>Impact (1-5)</span>
              <input type="number" min={1} max={5} value={draft.impact} onChange={(event) => setDraft((prev) => ({ ...prev, impact: Number(event.target.value) || 1 }))} />
            </label>
            <label>
              <span>Effort (1-5)</span>
              <input type="number" min={1} max={5} value={draft.effort} onChange={(event) => setDraft((prev) => ({ ...prev, effort: Number(event.target.value) || 1 }))} />
            </label>
            <label>
              <span>Risk (1-5)</span>
              <input type="number" min={1} max={5} value={draft.risk} onChange={(event) => setDraft((prev) => ({ ...prev, risk: Number(event.target.value) || 1 }))} />
            </label>
            <label>
              <span>Urgency (1-5)</span>
              <input type="number" min={1} max={5} value={draft.urgency} onChange={(event) => setDraft((prev) => ({ ...prev, urgency: Number(event.target.value) || 1 }))} />
            </label>
          </div>
          <label>
            <span>Tags (comma-separated)</span>
            <input value={draftTags} onChange={(event) => setDraftTags(event.target.value)} />
          </label>
          <label>
            <span>Notes</span>
            <textarea rows={3} value={draft.notes} onChange={(event) => setDraft((prev) => ({ ...prev, notes: event.target.value }))} />
          </label>
          <div className="settings-actions">
            <button className="primary-btn" disabled={busyItemID === (editTarget?.id ?? 'modal-create')} type="submit">
              {editTarget ? 'Save Changes' : 'Create'}
            </button>
            <button
              className="secondary-btn"
              onClick={() => {
                setEditTarget(null)
                setModalOpen(false)
              }}
              type="button"
            >
              Cancel
            </button>
          </div>
        </form>
      </Modal>
    </section>
  )
}
