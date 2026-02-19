import { useCallback, useEffect, useMemo, useState } from 'react'
import { Filter, Lightbulb, Pencil, Rocket, Trash2 } from 'lucide-react'
import {
  createDemandPoolItem,
  deleteDemandPoolItem,
  listDemandPoolItems,
  listProjects,
  promoteDemandPoolItem,
  updateDemandPoolItem,
} from '../api/client'
import type { DemandPoolItem, DemandPoolStatus, Project } from '../api/types'
import Modal from '../components/Modal'

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

export default function DemandPool() {
  const [projects, setProjects] = useState<Project[]>([])
  const [projectID, setProjectID] = useState('')
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

  const selectedProject = useMemo(() => projects.find((project) => project.id === projectID) ?? null, [projects, projectID])

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
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load demand pool items')
    } finally {
      setLoading(false)
    }
  }, [projectID, queryFilter, statusFilter])

  useEffect(() => {
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
  }, [])

  useEffect(() => {
    void loadDemandItems()
  }, [loadDemandItems])

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

  return (
    <section className="page-block demand-page">
      <div className="demand-header">
        <div>
          <h2>Demand Pool</h2>
          <p>Capture ideas and prioritize them separately from active development execution.</p>
        </div>
        <button className="primary-btn" onClick={openCreateModal} type="button">
          <Lightbulb size={16} />
          <span>Rich Add</span>
        </button>
      </div>

      <div className="dashboard-section demand-controls">
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

      <div className="dashboard-section demand-quick-add">
        <label>
          <span>Quick Add</span>
          <input
            placeholder={selectedProject ? `Add an idea for ${selectedProject.name}` : 'Add an idea'}
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

      {(message || error) && (
        <p className={`settings-message ${error ? 'text-error' : ''}`.trim()}>{error || message}</p>
      )}

      <div className="dashboard-section demand-list">
        {loading ? (
          <p className="empty-text">Loading demand pool...</p>
        ) : items.length === 0 ? (
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
                {items.map((item) => (
                  <tr key={item.id}>
                    <td>
                      <strong>{item.title}</strong>
                      {item.description ? <p>{item.description}</p> : null}
                    </td>
                    <td>{item.status}</td>
                    <td>{item.priority}</td>
                    <td>
                      {item.impact}/{item.effort}
                    </td>
                    <td>{item.tags.join(', ') || '-'}</td>
                    <td>
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
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
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
