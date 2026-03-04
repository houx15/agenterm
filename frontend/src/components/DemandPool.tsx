import { useCallback, useEffect, useState } from 'react'
import { useAppContext } from '../App'
import {
  listRequirements,
  createRequirement,
  updateRequirement,
  deleteRequirement,
  reorderRequirements,
} from '../api/client'
import type { Requirement } from '../api/client'
import { ArrowUp, ArrowDown, Pencil, Trash2, ChevronDown, ChevronRight } from './Lucide'

type RequirementStatus = 'queued' | 'planning' | 'building' | 'reviewing' | 'testing' | 'done' | string

function statusBadgeClasses(status: RequirementStatus): string {
  switch (status) {
    case 'queued':
      return 'bg-status-idle/20 text-status-idle'
    case 'planning':
      return 'bg-blue-500/20 text-blue-400'
    case 'building':
      return 'bg-status-working/20 text-status-working'
    case 'reviewing':
      return 'bg-status-waiting/20 text-status-waiting'
    case 'testing':
      return 'bg-purple-500/20 text-purple-400'
    case 'done':
      return 'bg-status-working/20 text-status-working'
    default:
      return 'bg-status-idle/20 text-status-idle'
  }
}

function statusLabel(status: RequirementStatus): string {
  switch (status) {
    case 'done':
      return 'Done'
    default:
      return status.charAt(0).toUpperCase() + status.slice(1)
  }
}

export default function DemandPool() {
  const { selectedProjectID, setMode } = useAppContext()
  const [requirements, setRequirements] = useState<Requirement[]>([])
  const [newTitle, setNewTitle] = useState('')
  const [editingID, setEditingID] = useState<string | null>(null)
  const [editTitle, setEditTitle] = useState('')
  const [expandedDone, setExpandedDone] = useState<Record<string, boolean>>({})
  const [loading, setLoading] = useState(false)

  const fetchRequirements = useCallback(async () => {
    if (!selectedProjectID) {
      setRequirements([])
      return
    }
    try {
      const list = await listRequirements(selectedProjectID)
      setRequirements(list)
    } catch {
      // keep UI usable
    }
  }, [selectedProjectID])

  useEffect(() => {
    void fetchRequirements()
  }, [fetchRequirements])

  const handleAdd = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!newTitle.trim() || !selectedProjectID) return
    setLoading(true)
    try {
      await createRequirement(selectedProjectID, { title: newTitle.trim() })
      setNewTitle('')
      await fetchRequirements()
    } catch {
      // keep UI usable
    } finally {
      setLoading(false)
    }
  }

  const handleDelete = async (id: string) => {
    try {
      await deleteRequirement(id)
      await fetchRequirements()
    } catch {
      // keep UI usable
    }
  }

  const handleEditSave = async (id: string) => {
    if (!editTitle.trim()) return
    try {
      await updateRequirement(id, { title: editTitle.trim() })
      setEditingID(null)
      setEditTitle('')
      await fetchRequirements()
    } catch {
      // keep UI usable
    }
  }

  const handleReorder = async (index: number, direction: 'up' | 'down') => {
    if (!selectedProjectID) return
    const newList = [...requirements]
    const targetIndex = direction === 'up' ? index - 1 : index + 1
    if (targetIndex < 0 || targetIndex >= newList.length) return

    ;[newList[index], newList[targetIndex]] = [newList[targetIndex], newList[index]]
    setRequirements(newList)

    try {
      await reorderRequirements(
        selectedProjectID,
        newList.map((r) => r.id),
      )
    } catch {
      await fetchRequirements()
    }
  }

  const handleRowClick = (req: Requirement) => {
    if (req.status !== 'queued' && req.status !== 'done') {
      setMode('workspace')
    }
  }

  if (!selectedProjectID) {
    return (
      <div className="flex items-center justify-center h-full text-text-secondary">
        Select a project to manage requirements
      </div>
    )
  }

  const activeRequirements = requirements.filter((r) => r.status !== 'done')
  const doneRequirements = requirements.filter((r) => r.status === 'done')

  return (
    <div className="flex flex-col h-full overflow-hidden">
      {/* Add new demand */}
      <form
        onSubmit={(e) => void handleAdd(e)}
        className="flex gap-2 p-4 border-b border-border shrink-0"
      >
        <input
          type="text"
          value={newTitle}
          onChange={(e) => setNewTitle(e.target.value)}
          placeholder="What do you want to build?"
          className="flex-1 rounded border border-border bg-bg-tertiary px-3 py-2 text-sm text-text-primary placeholder:text-text-secondary/50 focus:border-accent focus:outline-none"
          disabled={loading}
        />
        <button
          type="submit"
          disabled={loading || !newTitle.trim()}
          className="rounded bg-accent px-4 py-2 text-sm font-medium text-white hover:bg-accent/80 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
        >
          Add
        </button>
      </form>

      {/* Requirements table */}
      <div className="flex-1 overflow-y-auto">
        {requirements.length === 0 ? (
          <div className="flex items-center justify-center h-full text-text-secondary text-sm">
            No requirements yet. Add one above to get started.
          </div>
        ) : (
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-border text-text-secondary text-left">
                <th className="px-4 py-2 w-12 font-medium">#</th>
                <th className="px-4 py-2 font-medium">Title</th>
                <th className="px-4 py-2 w-28 font-medium">Status</th>
                <th className="px-4 py-2 w-32 font-medium text-right">Actions</th>
              </tr>
            </thead>
            <tbody>
              {activeRequirements.map((req, index) => (
                <tr
                  key={req.id}
                  className="border-b border-border/50 hover:bg-bg-tertiary/50 cursor-pointer transition-colors"
                  onClick={() => handleRowClick(req)}
                >
                  <td className="px-4 py-2 text-text-secondary tabular-nums">{req.priority || index + 1}</td>
                  <td className="px-4 py-2">
                    {editingID === req.id ? (
                      <input
                        type="text"
                        value={editTitle}
                        onChange={(e) => setEditTitle(e.target.value)}
                        onKeyDown={(e) => {
                          if (e.key === 'Enter') void handleEditSave(req.id)
                          if (e.key === 'Escape') setEditingID(null)
                        }}
                        onBlur={() => void handleEditSave(req.id)}
                        className="w-full rounded border border-border bg-bg-tertiary px-2 py-1 text-sm text-text-primary focus:border-accent focus:outline-none"
                        autoFocus
                        onClick={(e) => e.stopPropagation()}
                      />
                    ) : (
                      <span className="text-text-primary">{req.title}</span>
                    )}
                  </td>
                  <td className="px-4 py-2">
                    <span
                      className={`inline-block rounded-full px-2 py-0.5 text-xs font-medium ${statusBadgeClasses(req.status)}`}
                    >
                      {statusLabel(req.status)}
                    </span>
                  </td>
                  <td className="px-4 py-2">
                    <div className="flex items-center justify-end gap-1" onClick={(e) => e.stopPropagation()}>
                      <button
                        className="p-1 rounded text-text-secondary hover:text-text-primary hover:bg-bg-tertiary transition-colors disabled:opacity-30"
                        onClick={() => void handleReorder(index, 'up')}
                        disabled={index === 0}
                        title="Move up"
                        type="button"
                      >
                        <ArrowUp size={14} />
                      </button>
                      <button
                        className="p-1 rounded text-text-secondary hover:text-text-primary hover:bg-bg-tertiary transition-colors disabled:opacity-30"
                        onClick={() => void handleReorder(index, 'down')}
                        disabled={index === activeRequirements.length - 1}
                        title="Move down"
                        type="button"
                      >
                        <ArrowDown size={14} />
                      </button>
                      <button
                        className="p-1 rounded text-text-secondary hover:text-text-primary hover:bg-bg-tertiary transition-colors"
                        onClick={() => {
                          setEditingID(req.id)
                          setEditTitle(req.title)
                        }}
                        title="Edit"
                        type="button"
                      >
                        <Pencil size={14} />
                      </button>
                      <button
                        className="p-1 rounded text-text-secondary hover:text-status-error hover:bg-bg-tertiary transition-colors"
                        onClick={() => void handleDelete(req.id)}
                        title="Delete"
                        type="button"
                      >
                        <Trash2 size={14} />
                      </button>
                    </div>
                  </td>
                </tr>
              ))}

              {/* Done section */}
              {doneRequirements.length > 0 && (
                <>
                  <tr className="border-b border-border/50">
                    <td colSpan={4} className="px-4 py-2">
                      <button
                        className="flex items-center gap-1 text-xs text-text-secondary hover:text-text-primary transition-colors"
                        onClick={() => setExpandedDone((prev) => ({ ...prev, __all: !prev.__all }))}
                        type="button"
                      >
                        {expandedDone.__all ? <ChevronDown size={12} /> : <ChevronRight size={12} />}
                        Completed ({doneRequirements.length})
                      </button>
                    </td>
                  </tr>
                  {expandedDone.__all &&
                    doneRequirements.map((req) => (
                      <tr
                        key={req.id}
                        className="border-b border-border/50 opacity-60"
                      >
                        <td className="px-4 py-2 text-text-secondary tabular-nums">{req.priority}</td>
                        <td className="px-4 py-2 text-text-secondary line-through">{req.title}</td>
                        <td className="px-4 py-2">
                          <span className={`inline-block rounded-full px-2 py-0.5 text-xs font-medium ${statusBadgeClasses('done')}`}>
                            Done
                          </span>
                        </td>
                        <td className="px-4 py-2" />
                      </tr>
                    ))}
                </>
              )}
            </tbody>
          </table>
        )}
      </div>
    </div>
  )
}
