import { useEffect, useMemo, useState } from 'react'
import { CircleAlert, CircleCheck, Plus, Trash2 } from 'lucide-react'
import { createAgent, deleteAgent, listAgents, updateAgent } from '../api/client'
import type { AgentConfig } from '../api/types'
import Modal from '../components/Modal'

type NoticeKind = 'success' | 'error'

const DEFAULT_AGENT: AgentConfig = {
  id: '',
  name: '',
  model: '',
  command: '',
  max_parallel_agents: 1,
  supports_orchestrator: false,
  orchestrator_provider: 'anthropic',
  orchestrator_api_key: '',
  orchestrator_api_base: '',
  capabilities: [],
  languages: [],
  cost_tier: 'medium',
  speed_tier: 'medium',
  supports_session_resume: false,
  supports_headless: false,
  notes: '',
}

function clampParallelAgents(value: number): number {
  if (!Number.isFinite(value)) {
    return 1
  }
  return Math.min(64, Math.max(1, Math.trunc(value)))
}

export default function Settings() {
  const [agents, setAgents] = useState<AgentConfig[]>([])
  const [selectedID, setSelectedID] = useState('')
  const [draft, setDraft] = useState<AgentConfig>(DEFAULT_AGENT)
  const [loading, setLoading] = useState(true)
  const [busy, setBusy] = useState(false)
  const [notice, setNotice] = useState<{ kind: NoticeKind; text: string } | null>(null)
  const [deleteModalOpen, setDeleteModalOpen] = useState(false)

  const isNew = selectedID === ''
  const selectedAgent = useMemo(() => agents.find((item) => item.id === selectedID) ?? null, [agents, selectedID])

  useEffect(() => {
    let cancelled = false
    const load = async () => {
      setLoading(true)
      try {
        const items = await listAgents<AgentConfig[]>()
        if (cancelled) {
          return
        }
        setAgents(items)
        if (items.length > 0) {
          setSelectedID(items[0].id)
          setDraft(items[0])
        }
      } catch (err) {
        if (!cancelled) {
          setNotice({ kind: 'error', text: err instanceof Error ? err.message : 'Failed to load agents' })
        }
      } finally {
        if (!cancelled) {
          setLoading(false)
        }
      }
    }
    void load()
    return () => {
      cancelled = true
    }
  }, [])

  const selectAgent = (id: string) => {
    const found = agents.find((item) => item.id === id)
    if (!found) {
      return
    }
    setSelectedID(id)
    setDraft(found)
    setNotice(null)
  }

  const startNewAgent = () => {
    setSelectedID('')
    setDraft(DEFAULT_AGENT)
    setNotice(null)
  }

  const saveAgent = async () => {
    if (!draft.id.trim()) {
      setNotice({ kind: 'error', text: 'Agent ID is required.' })
      return
    }
    if (!draft.name.trim()) {
      setNotice({ kind: 'error', text: 'Agent name is required.' })
      return
    }
    if (!draft.command.trim()) {
      setNotice({ kind: 'error', text: 'Agent command is required.' })
      return
    }

    setBusy(true)
    setNotice(null)
    try {
      if (isNew) {
        const created = await createAgent<AgentConfig>(draft)
        const next = [...agents.filter((item) => item.id !== created.id), created].sort((a, b) => a.name.localeCompare(b.name))
        setAgents(next)
        setSelectedID(created.id)
        setDraft(created)
      } else {
        const updated = await updateAgent<AgentConfig>(selectedID, draft)
        setAgents((prev) => prev.map((item) => (item.id === updated.id ? updated : item)))
        setDraft(updated)
      }
      setNotice({ kind: 'success', text: 'Agent saved.' })
    } catch (err) {
      setNotice({ kind: 'error', text: err instanceof Error ? err.message : 'Failed to save agent' })
    } finally {
      setBusy(false)
    }
  }

  const removeAgent = async () => {
    if (!selectedID) {
      return
    }
    setBusy(true)
    setNotice(null)
    try {
      await deleteAgent(selectedID)
      const next = agents.filter((item) => item.id !== selectedID)
      setAgents(next)
      if (next.length > 0) {
        setSelectedID(next[0].id)
        setDraft(next[0])
      } else {
        setSelectedID('')
        setDraft(DEFAULT_AGENT)
      }
      setNotice({ kind: 'success', text: 'Agent deleted.' })
      setDeleteModalOpen(false)
    } catch (err) {
      setNotice({ kind: 'error', text: err instanceof Error ? err.message : 'Failed to delete agent' })
    } finally {
      setBusy(false)
    }
  }

  if (loading) {
    return (
      <section className="page-block settings-page">
        <h2>Agent Registry</h2>
        <p>Loading agents...</p>
      </section>
    )
  }

  return (
    <section className="page-block settings-page">
      <div className="settings-header-row">
        <h2>Agent Registry</h2>
        <button className="primary-btn" onClick={startNewAgent} type="button">
          <Plus size={14} />
          <span>New Agent</span>
        </button>
      </div>

      <p className="empty-text">Register local TUIs with one-line specs. Orchestrator picks workers from this list.</p>

      {notice ? (
        <div className={`settings-notice ${notice.kind}`.trim()} role="status" aria-live="polite">
          {notice.kind === 'success' ? <CircleCheck size={14} /> : <CircleAlert size={14} />}
          <span>{notice.text}</span>
        </div>
      ) : null}

      <div className="settings-grid">
        <aside className="settings-list">
          {isNew ? (
            <button type="button" className="session-row active">
              <strong>{draft.name.trim() || 'New Agent (Draft)'}</strong>
              <small>{draft.id.trim() || 'unsaved'}</small>
            </button>
          ) : null}
          {agents.map((item) => (
            <button
              key={item.id}
              type="button"
              className={`session-row ${item.id === selectedID ? 'active' : ''}`.trim()}
              onClick={() => selectAgent(item.id)}
            >
              <strong>{item.name}</strong>
              <small>{item.id}</small>
            </button>
          ))}
        </aside>

        <div className="settings-editor">
          <label>
            Agent ID
            <input
              value={draft.id}
              disabled={!isNew}
              onChange={(event) => setDraft((prev) => ({ ...prev, id: event.target.value.trim().toLowerCase() }))}
              placeholder="codex-worker"
            />
          </label>
          <label>
            Display Name
            <input value={draft.name} onChange={(event) => setDraft((prev) => ({ ...prev, name: event.target.value }))} placeholder="Codex Worker" />
          </label>
          <label>
            One-line Spec
            <input
              value={draft.notes ?? ''}
              onChange={(event) => setDraft((prev) => ({ ...prev, notes: event.target.value }))}
              placeholder="TypeScript full-stack, strong reviewer, prefers repo-wide refactors."
            />
          </label>
          <label>
            Start Command
            <textarea
              rows={4}
              value={draft.command}
              onChange={(event) => setDraft((prev) => ({ ...prev, command: event.target.value }))}
              placeholder={`cd {worktree_path}\nclaude --dangerously-skip-permissions`}
            />
          </label>
          <label>
            Session Model (optional)
            <input
              value={draft.model ?? ''}
              onChange={(event) => setDraft((prev) => ({ ...prev, model: event.target.value }))}
              placeholder="claude-sonnet-4-5 / gpt-5-codex"
            />
          </label>
          <label>
            Max Parallel Instances
            <input
              type="number"
              min={1}
              max={64}
              value={draft.max_parallel_agents ?? 1}
              onChange={(event) =>
                setDraft((prev) => ({
                  ...prev,
                  max_parallel_agents: clampParallelAgents(Number(event.target.value || 1)),
                }))
              }
            />
          </label>

          <label className="settings-field-checkbox">
            <span>Can be used as orchestrator model</span>
            <input
              type="checkbox"
              checked={!!draft.supports_orchestrator}
              onChange={(event) =>
                setDraft((prev) => ({
                  ...prev,
                  supports_orchestrator: event.target.checked,
                  orchestrator_provider: prev.orchestrator_provider || 'anthropic',
                }))
              }
            />
          </label>

          {draft.supports_orchestrator ? (
            <>
              <label>
                Orchestrator Provider
                <select
                  value={draft.orchestrator_provider ?? 'anthropic'}
                  onChange={(event) =>
                    setDraft((prev) => ({ ...prev, orchestrator_provider: event.target.value as 'anthropic' | 'openai' }))
                  }
                >
                  <option value="anthropic">anthropic</option>
                  <option value="openai">openai</option>
                </select>
              </label>
              <label>
                Orchestrator API Endpoint
                <input
                  value={draft.orchestrator_api_base ?? ''}
                  onChange={(event) => setDraft((prev) => ({ ...prev, orchestrator_api_base: event.target.value }))}
                  placeholder="https://api.anthropic.com/v1/messages"
                />
              </label>
              <label>
                Orchestrator API Key
                <input
                  type="password"
                  value={draft.orchestrator_api_key ?? ''}
                  onChange={(event) => setDraft((prev) => ({ ...prev, orchestrator_api_key: event.target.value }))}
                  placeholder="sk-..."
                />
              </label>
            </>
          ) : null}

          <div className="settings-actions">
            <button className="primary-btn" disabled={busy} onClick={() => void saveAgent()} type="button">
              Save Agent
            </button>
            {!isNew ? (
              <button className="action-btn danger" disabled={busy} onClick={() => setDeleteModalOpen(true)} type="button">
                Delete
              </button>
            ) : null}
          </div>
        </div>
      </div>

      <Modal onClose={() => setDeleteModalOpen(false)} open={deleteModalOpen} title="Delete Agent">
        <div className="modal-form">
          <p className="empty-text">
            Delete agent <strong>{selectedAgent?.name ?? ''}</strong>?
          </p>
          <div className="settings-actions">
            <button className="secondary-btn" disabled={busy} onClick={() => setDeleteModalOpen(false)} type="button">
              Cancel
            </button>
            <button className="secondary-btn danger-btn" disabled={busy} onClick={() => void removeAgent()} type="button">
              <Trash2 size={14} />
              <span>Delete</span>
            </button>
          </div>
        </div>
      </Modal>
    </section>
  )
}
