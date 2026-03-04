import { useEffect, useMemo, useState } from 'react'
import {
  createAgent,
  deleteAgent,
  listAgents,
  updateAgent,
  listPermissionTemplates,
  createPermissionTemplate,
  updatePermissionTemplate,
  deletePermissionTemplate,
  getToken,
} from '../api/client'
import type { AgentConfig } from '../api/types'
import type { PermissionTemplate } from '../api/client'
import Modal from './Modal'
import { Plus, Trash2 } from './Lucide'

// ---------------------------------------------------------------------------
// Props & helpers
// ---------------------------------------------------------------------------

interface SettingsModalProps {
  open: boolean
  onClose: () => void
}

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
  if (!Number.isFinite(value)) return 1
  return Math.min(64, Math.max(1, Math.trunc(value)))
}

type SettingsTab = 'agents' | 'permissions' | 'general'

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

export default function SettingsModal({ open, onClose }: SettingsModalProps) {
  const [activeTab, setActiveTab] = useState<SettingsTab>('agents')

  // ── Agent Registry state ──
  const [agents, setAgents] = useState<AgentConfig[]>([])
  const [selectedID, setSelectedID] = useState('')
  const [draft, setDraft] = useState<AgentConfig>(DEFAULT_AGENT)
  const [loading, setLoading] = useState(true)
  const [busy, setBusy] = useState(false)
  const [notice, setNotice] = useState<{ kind: NoticeKind; text: string } | null>(null)
  const [deleteConfirm, setDeleteConfirm] = useState(false)

  // ── Permission Templates state ──
  const [templates, setTemplates] = useState<PermissionTemplate[]>([])
  const [templatesLoading, setTemplatesLoading] = useState(true)
  const [selectedTemplateID, setSelectedTemplateID] = useState('')
  const [templateDraft, setTemplateDraft] = useState({ agent_type: '', name: '', config: '' })
  const [templateNotice, setTemplateNotice] = useState<{ kind: NoticeKind; text: string } | null>(null)
  const [templateBusy, setTemplateBusy] = useState(false)
  const [templateDeleteConfirm, setTemplateDeleteConfirm] = useState(false)

  const isNew = selectedID === ''
  const isNewTemplate = selectedTemplateID === ''
  const selectedAgent = useMemo(
    () => agents.find((item) => item.id === selectedID) ?? null,
    [agents, selectedID],
  )

  // ── Load agents ──
  useEffect(() => {
    if (!open) return
    let cancelled = false
    const load = async () => {
      setLoading(true)
      try {
        const items = await listAgents<AgentConfig[]>()
        if (cancelled) return
        setAgents(items)
        if (items.length > 0) {
          setSelectedID(items[0].id)
          setDraft(items[0])
        } else {
          setSelectedID('')
          setDraft(DEFAULT_AGENT)
        }
      } catch (err) {
        if (!cancelled) {
          setNotice({ kind: 'error', text: err instanceof Error ? err.message : 'Failed to load agents' })
        }
      } finally {
        if (!cancelled) setLoading(false)
      }
    }
    void load()
    return () => { cancelled = true }
  }, [open])

  // ── Load permission templates ──
  useEffect(() => {
    if (!open) return
    let cancelled = false
    const load = async () => {
      setTemplatesLoading(true)
      try {
        const items = await listPermissionTemplates()
        if (cancelled) return
        setTemplates(items)
        if (items.length > 0) {
          setSelectedTemplateID(items[0].id)
          setTemplateDraft({ agent_type: items[0].agent_type, name: items[0].name, config: items[0].config })
        } else {
          setSelectedTemplateID('')
          setTemplateDraft({ agent_type: '', name: '', config: '' })
        }
      } catch {
        // non-critical
      } finally {
        if (!cancelled) setTemplatesLoading(false)
      }
    }
    void load()
    return () => { cancelled = true }
  }, [open])

  // ── Agent handlers ──
  const selectAgent = (id: string) => {
    const found = agents.find((item) => item.id === id)
    if (!found) return
    setSelectedID(id)
    setDraft(found)
    setNotice(null)
    setDeleteConfirm(false)
  }

  const startNewAgent = () => {
    setSelectedID('')
    setDraft(DEFAULT_AGENT)
    setNotice(null)
    setDeleteConfirm(false)
  }

  const saveAgent = async () => {
    if (!draft.id.trim()) { setNotice({ kind: 'error', text: 'Agent ID is required.' }); return }
    if (!draft.name.trim()) { setNotice({ kind: 'error', text: 'Agent name is required.' }); return }
    if (!draft.command.trim()) { setNotice({ kind: 'error', text: 'Agent command is required.' }); return }

    setBusy(true)
    setNotice(null)
    try {
      if (isNew) {
        const created = await createAgent<AgentConfig>(draft)
        const next = [...agents.filter((item) => item.id !== created.id), created].sort((a, b) =>
          a.name.localeCompare(b.name),
        )
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
    if (!selectedID) return
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
      setDeleteConfirm(false)
    } catch (err) {
      setNotice({ kind: 'error', text: err instanceof Error ? err.message : 'Failed to delete agent' })
    } finally {
      setBusy(false)
    }
  }

  // ── Permission template handlers ──
  const selectTemplate = (id: string) => {
    const found = templates.find((t) => t.id === id)
    if (!found) return
    setSelectedTemplateID(id)
    setTemplateDraft({ agent_type: found.agent_type, name: found.name, config: found.config })
    setTemplateNotice(null)
    setTemplateDeleteConfirm(false)
  }

  const startNewTemplate = () => {
    setSelectedTemplateID('')
    setTemplateDraft({ agent_type: '', name: '', config: '{"allow": [], "deny": []}' })
    setTemplateNotice(null)
    setTemplateDeleteConfirm(false)
  }

  const saveTemplate = async () => {
    if (!templateDraft.agent_type.trim()) { setTemplateNotice({ kind: 'error', text: 'Agent type is required.' }); return }
    if (!templateDraft.name.trim()) { setTemplateNotice({ kind: 'error', text: 'Template name is required.' }); return }

    setTemplateBusy(true)
    setTemplateNotice(null)
    try {
      if (isNewTemplate) {
        const created = await createPermissionTemplate({
          agent_type: templateDraft.agent_type.trim(),
          name: templateDraft.name.trim(),
          config: templateDraft.config,
        })
        setTemplates((prev) => [...prev, created])
        setSelectedTemplateID(created.id)
      } else {
        const updated = await updatePermissionTemplate(selectedTemplateID, {
          agent_type: templateDraft.agent_type.trim(),
          name: templateDraft.name.trim(),
          config: templateDraft.config,
        })
        setTemplates((prev) => prev.map((t) => (t.id === updated.id ? updated : t)))
      }
      setTemplateNotice({ kind: 'success', text: 'Template saved.' })
    } catch (err) {
      setTemplateNotice({ kind: 'error', text: err instanceof Error ? err.message : 'Failed to save template' })
    } finally {
      setTemplateBusy(false)
    }
  }

  const removeTemplate = async () => {
    if (!selectedTemplateID) return
    setTemplateBusy(true)
    setTemplateNotice(null)
    try {
      await deletePermissionTemplate(selectedTemplateID)
      const next = templates.filter((t) => t.id !== selectedTemplateID)
      setTemplates(next)
      if (next.length > 0) {
        setSelectedTemplateID(next[0].id)
        setTemplateDraft({ agent_type: next[0].agent_type, name: next[0].name, config: next[0].config })
      } else {
        setSelectedTemplateID('')
        setTemplateDraft({ agent_type: '', name: '', config: '' })
      }
      setTemplateNotice({ kind: 'success', text: 'Template deleted.' })
      setTemplateDeleteConfirm(false)
    } catch (err) {
      setTemplateNotice({ kind: 'error', text: err instanceof Error ? err.message : 'Failed to delete template' })
    } finally {
      setTemplateBusy(false)
    }
  }

  // ── Render helpers ──
  const renderNotice = (n: { kind: NoticeKind; text: string } | null) => {
    if (!n) return null
    return (
      <div
        role="status"
        aria-live="polite"
        style={{
          padding: '0.45rem 0.7rem',
          borderRadius: '8px',
          fontSize: '0.85rem',
          background: n.kind === 'success' ? 'var(--success-bg, #e8f5e9)' : 'var(--error-bg, #fdecea)',
          color: n.kind === 'success' ? 'var(--success, #2e7d32)' : 'var(--error, #c62828)',
        }}
      >
        {n.text}
      </div>
    )
  }

  return (
    <Modal open={open} title="Settings" onClose={onClose}>
      <div className="modal-form">
        {/* Tab bar */}
        <div className="settings-tabs">
          {(['agents', 'permissions', 'general'] as SettingsTab[]).map((tab) => (
            <button
              key={tab}
              className={`settings-tab ${activeTab === tab ? 'active' : ''}`.trim()}
              onClick={() => setActiveTab(tab)}
              type="button"
            >
              {tab === 'agents' ? 'Agent Registry' : tab === 'permissions' ? 'Permission Templates' : 'General'}
            </button>
          ))}
        </div>

        {/* ── Agent Registry Tab ── */}
        {activeTab === 'agents' && (
          <>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
              <p style={{ margin: 0, color: 'var(--text-secondary)', fontSize: '0.9rem' }}>
                Register CLI agents. The orchestrator picks workers from this list.
              </p>
              <button className="btn btn-primary" onClick={startNewAgent} type="button">
                <Plus size={14} />
                <span>New Agent</span>
              </button>
            </div>

            {renderNotice(notice)}

            {loading ? (
              <p style={{ color: 'var(--text-secondary)', fontSize: '0.9rem' }}>Loading agents...</p>
            ) : (
              <div className="settings-grid">
                <aside className="settings-list">
                  {isNew && (
                    <button type="button" className="session-row active">
                      <strong>{draft.name.trim() || 'New Agent (Draft)'}</strong>
                      <small>{draft.id.trim() || 'unsaved'}</small>
                    </button>
                  )}
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
                      onChange={(e) => setDraft((prev) => ({ ...prev, id: e.target.value.trim().toLowerCase() }))}
                      placeholder="codex-worker"
                    />
                  </label>
                  <label>
                    Display Name
                    <input
                      value={draft.name}
                      onChange={(e) => setDraft((prev) => ({ ...prev, name: e.target.value }))}
                      placeholder="Codex Worker"
                    />
                  </label>
                  <label>
                    One-line Spec
                    <input
                      value={draft.notes ?? ''}
                      onChange={(e) => setDraft((prev) => ({ ...prev, notes: e.target.value }))}
                      placeholder="TypeScript full-stack, strong reviewer."
                    />
                  </label>
                  <label>
                    Start Command
                    <textarea
                      rows={3}
                      value={draft.command}
                      onChange={(e) => setDraft((prev) => ({ ...prev, command: e.target.value }))}
                      placeholder={'cd {worktree_path}\nclaude --dangerously-skip-permissions'}
                    />
                  </label>
                  <label>
                    Max Parallel Instances
                    <input
                      type="number"
                      min={1}
                      max={64}
                      value={draft.max_parallel_agents ?? 1}
                      onChange={(e) =>
                        setDraft((prev) => ({
                          ...prev,
                          max_parallel_agents: clampParallelAgents(Number(e.target.value || 1)),
                        }))
                      }
                    />
                  </label>

                  <div className="settings-actions">
                    <button className="btn btn-primary" disabled={busy} onClick={() => void saveAgent()} type="button">
                      Save Agent
                    </button>
                    {!isNew && (
                      deleteConfirm ? (
                        <>
                          <span style={{ fontSize: '0.85rem', color: 'var(--text-secondary)' }}>
                            Delete <strong>{selectedAgent?.name}</strong>?
                          </span>
                          <button className="btn btn-danger" disabled={busy} onClick={() => void removeAgent()} type="button">
                            <Trash2 size={14} /> <span>Confirm</span>
                          </button>
                          <button className="btn btn-ghost" disabled={busy} onClick={() => setDeleteConfirm(false)} type="button">
                            Cancel
                          </button>
                        </>
                      ) : (
                        <button className="btn btn-ghost" disabled={busy} onClick={() => setDeleteConfirm(true)} type="button">
                          <Trash2 size={14} /> <span>Delete</span>
                        </button>
                      )
                    )}
                  </div>
                </div>
              </div>
            )}
          </>
        )}

        {/* ── Permission Templates Tab ── */}
        {activeTab === 'permissions' && (
          <>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
              <p style={{ margin: 0, color: 'var(--text-secondary)', fontSize: '0.9rem' }}>
                Define allowed and denied commands per agent type.
              </p>
              <button className="btn btn-primary" onClick={startNewTemplate} type="button">
                <Plus size={14} />
                <span>New Template</span>
              </button>
            </div>

            {renderNotice(templateNotice)}

            {templatesLoading ? (
              <p style={{ color: 'var(--text-secondary)', fontSize: '0.9rem' }}>Loading templates...</p>
            ) : (
              <div className="settings-grid">
                <aside className="settings-list">
                  {isNewTemplate && (
                    <button type="button" className="session-row active">
                      <strong>{templateDraft.name.trim() || 'New Template (Draft)'}</strong>
                      <small>{templateDraft.agent_type.trim() || 'unsaved'}</small>
                    </button>
                  )}
                  {templates.map((t) => (
                    <button
                      key={t.id}
                      type="button"
                      className={`session-row ${t.id === selectedTemplateID ? 'active' : ''}`.trim()}
                      onClick={() => selectTemplate(t.id)}
                    >
                      <strong>{t.name}</strong>
                      <small>{t.agent_type}</small>
                    </button>
                  ))}
                </aside>

                <div className="settings-editor">
                  <label>
                    Agent Type
                    <input
                      value={templateDraft.agent_type}
                      onChange={(e) => setTemplateDraft((prev) => ({ ...prev, agent_type: e.target.value }))}
                      placeholder="claude-code"
                    />
                  </label>
                  <label>
                    Template Name
                    <input
                      value={templateDraft.name}
                      onChange={(e) => setTemplateDraft((prev) => ({ ...prev, name: e.target.value }))}
                      placeholder="Standard permissions"
                    />
                  </label>
                  <label>
                    Config (JSON)
                    <textarea
                      rows={8}
                      value={templateDraft.config}
                      onChange={(e) => setTemplateDraft((prev) => ({ ...prev, config: e.target.value }))}
                      placeholder='{"allow": ["git *", "npm *"], "deny": ["rm -rf *", "sudo *"]}'
                      style={{ fontFamily: 'monospace' }}
                    />
                  </label>

                  <div className="settings-actions">
                    <button className="btn btn-primary" disabled={templateBusy} onClick={() => void saveTemplate()} type="button">
                      Save Template
                    </button>
                    {!isNewTemplate && (
                      templateDeleteConfirm ? (
                        <>
                          <span style={{ fontSize: '0.85rem', color: 'var(--text-secondary)' }}>Delete this template?</span>
                          <button className="btn btn-danger" disabled={templateBusy} onClick={() => void removeTemplate()} type="button">
                            <Trash2 size={14} /> <span>Confirm</span>
                          </button>
                          <button className="btn btn-ghost" disabled={templateBusy} onClick={() => setTemplateDeleteConfirm(false)} type="button">
                            Cancel
                          </button>
                        </>
                      ) : (
                        <button className="btn btn-ghost" disabled={templateBusy} onClick={() => setTemplateDeleteConfirm(true)} type="button">
                          <Trash2 size={14} /> <span>Delete</span>
                        </button>
                      )
                    )}
                  </div>
                </div>
              </div>
            )}
          </>
        )}

        {/* ── General Tab ── */}
        {activeTab === 'general' && (
          <div className="settings-editor" style={{ maxWidth: '480px' }}>
            <p style={{ margin: '0 0 12px', color: 'var(--text-secondary)', fontSize: '0.9rem' }}>
              General application settings.
            </p>
            <label>
              Auth Token
              <input
                value={getToken()}
                readOnly
                onClick={(e) => (e.target as HTMLInputElement).select()}
                style={{ fontFamily: 'monospace', cursor: 'pointer' }}
              />
            </label>
            <p style={{ margin: '4px 0 0', color: 'var(--text-secondary)', fontSize: '0.8rem' }}>
              Click to select. This token authenticates the frontend with the backend.
            </p>
          </div>
        )}
      </div>
    </Modal>
  )
}
