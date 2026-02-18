import { useEffect, useMemo, useState } from 'react'
import {
  createAgent,
  createPlaybook,
  deleteAgent,
  deletePlaybook,
  listAgents,
  listPlaybooks,
  updateAgent,
  updatePlaybook,
} from '../api/client'
import type { AgentConfig, Playbook, PlaybookPhase } from '../api/types'

type TabKey = 'agents' | 'playbooks'
type PhaseEditorMode = 'table' | 'raw'

const DEFAULT_AGENT: AgentConfig = {
  id: '',
  name: '',
  model: 'default',
  command: '',
  max_parallel_agents: 1,
  capabilities: [],
  languages: [],
  cost_tier: 'medium',
  speed_tier: 'medium',
  supports_session_resume: false,
  supports_headless: false,
  notes: '',
}

const DEFAULT_PLAYBOOK: Playbook = {
  id: '',
  name: '',
  description: '',
  parallelism_strategy: '',
  match: { languages: [], project_patterns: [] },
  phases: [{ name: 'Implement', agent: 'codex', role: 'implementer', description: '' }],
}

function parseCSV(value: string): string[] {
  return value
    .split(',')
    .map((item) => item.trim())
    .filter(Boolean)
}

function stringifyPhases(phases: PlaybookPhase[]): string {
  return JSON.stringify(phases, null, 2)
}

function parseRawPhases(value: string): { phases: PlaybookPhase[]; error?: string } {
  let parsed: unknown
  try {
    parsed = JSON.parse(value)
  } catch {
    return { phases: [], error: 'Phases must be valid JSON.' }
  }

  if (!Array.isArray(parsed)) {
    return { phases: [], error: 'Raw phases must be a JSON array.' }
  }

  const phases: PlaybookPhase[] = []
  for (let i = 0; i < parsed.length; i += 1) {
    const item = parsed[i]
    if (!item || typeof item !== 'object' || Array.isArray(item)) {
      return { phases: [], error: `Phase ${i + 1} must be an object.` }
    }
    const record = item as Record<string, unknown>
    const name = typeof record.name === 'string' ? record.name.trim() : ''
    const agent = typeof record.agent === 'string' ? record.agent.trim() : ''
    const role = typeof record.role === 'string' ? record.role.trim() : ''
    const description = typeof record.description === 'string' ? record.description : ''
    if (!name || !agent || !role) {
      return { phases: [], error: `Phase ${i + 1} requires non-empty name, agent, and role.` }
    }
    phases.push({ name, agent, role, description })
  }

  return { phases }
}

export default function Settings() {
  const [activeTab, setActiveTab] = useState<TabKey>('agents')
  const [agents, setAgents] = useState<AgentConfig[]>([])
  const [playbooks, setPlaybooks] = useState<Playbook[]>([])
  const [selectedAgentID, setSelectedAgentID] = useState<string>('')
  const [selectedPlaybookID, setSelectedPlaybookID] = useState<string>('')
  const [agentDraft, setAgentDraft] = useState<AgentConfig>(DEFAULT_AGENT)
  const [playbookDraft, setPlaybookDraft] = useState<Playbook>(DEFAULT_PLAYBOOK)
  const [phasesEditor, setPhasesEditor] = useState<string>(stringifyPhases(DEFAULT_PLAYBOOK.phases))
  const [phaseEditorMode, setPhaseEditorMode] = useState<PhaseEditorMode>('table')
  const [loading, setLoading] = useState<boolean>(true)
  const [busy, setBusy] = useState<boolean>(false)
  const [message, setMessage] = useState<string>('')

  const isNewAgent = selectedAgentID === ''
  const isNewPlaybook = selectedPlaybookID === ''

  useEffect(() => {
    let cancelled = false
    async function load() {
      setLoading(true)
      try {
        const [agentsData, playbooksData] = await Promise.all([listAgents<AgentConfig[]>(), listPlaybooks<Playbook[]>()])
        if (cancelled) {
          return
        }
        setAgents(agentsData)
        setPlaybooks(playbooksData)
        if (agentsData.length > 0) {
          setSelectedAgentID(agentsData[0].id)
          setAgentDraft(agentsData[0])
        }
        if (playbooksData.length > 0) {
          setSelectedPlaybookID(playbooksData[0].id)
          setPlaybookDraft(playbooksData[0])
          setPhasesEditor(stringifyPhases(playbooksData[0].phases))
        }
      } catch (error) {
        setMessage(error instanceof Error ? error.message : 'Failed to load settings')
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

  const selectedAgent = useMemo(() => agents.find((item) => item.id === selectedAgentID) ?? null, [agents, selectedAgentID])
  const selectedPlaybook = useMemo(() => playbooks.find((item) => item.id === selectedPlaybookID) ?? null, [playbooks, selectedPlaybookID])

  function startNewAgent() {
    setSelectedAgentID('')
    setAgentDraft(DEFAULT_AGENT)
    setMessage('')
  }

  function startNewPlaybook() {
    setSelectedPlaybookID('')
    setPlaybookDraft(DEFAULT_PLAYBOOK)
    setPhasesEditor(stringifyPhases(DEFAULT_PLAYBOOK.phases))
    setMessage('')
  }

  function selectAgent(id: string) {
    const found = agents.find((item) => item.id === id)
    if (!found) {
      return
    }
    setSelectedAgentID(id)
    setAgentDraft(found)
    setMessage('')
  }

  function selectPlaybook(id: string) {
    const found = playbooks.find((item) => item.id === id)
    if (!found) {
      return
    }
    setSelectedPlaybookID(id)
    setPlaybookDraft(found)
    setPhasesEditor(stringifyPhases(found.phases))
    setMessage('')
  }

  function setDraftPhases(phases: PlaybookPhase[]) {
    setPlaybookDraft((prev) => ({ ...prev, phases }))
    setPhasesEditor(stringifyPhases(phases))
  }

  function updatePhase(index: number, patch: Partial<PlaybookPhase>) {
    const next = playbookDraft.phases.map((phase, i) => (i === index ? { ...phase, ...patch } : phase))
    setDraftPhases(next)
  }

  function addPhase() {
    const next = [...playbookDraft.phases, { name: '', agent: '', role: '', description: '' }]
    setDraftPhases(next)
  }

  function removePhase(index: number) {
    const next = playbookDraft.phases.filter((_, i) => i !== index)
    setDraftPhases(next.length > 0 ? next : [{ name: '', agent: '', role: '', description: '' }])
  }

  function switchPhaseEditorMode(mode: PhaseEditorMode) {
    if (mode === phaseEditorMode) {
      return
    }
    if (mode === 'raw') {
      setPhasesEditor(stringifyPhases(playbookDraft.phases))
      setPhaseEditorMode('raw')
      return
    }
    const parsed = parseRawPhases(phasesEditor)
    if (parsed.error) {
      setMessage(parsed.error)
      return
    }
    setDraftPhases(parsed.phases)
    setPhaseEditorMode('table')
    setMessage('')
  }

  async function saveAgent() {
    setBusy(true)
    setMessage('')
    try {
      if (isNewAgent) {
        const created = await createAgent<AgentConfig>(agentDraft)
        setAgents((prev) => [...prev.filter((item) => item.id !== created.id), created].sort((a, b) => a.name.localeCompare(b.name)))
        setSelectedAgentID(created.id)
        setAgentDraft(created)
      } else {
        const updated = await updateAgent<AgentConfig>(selectedAgentID, agentDraft)
        setAgents((prev) => prev.map((item) => (item.id === updated.id ? updated : item)))
        setAgentDraft(updated)
      }
      setMessage('Agent saved.')
    } catch (error) {
      setMessage(error instanceof Error ? error.message : 'Failed to save agent')
    } finally {
      setBusy(false)
    }
  }

  async function removeAgent() {
    if (!selectedAgentID || !selectedAgent) {
      return
    }
    if (!window.confirm(`Delete agent ${selectedAgent.name}?`)) {
      return
    }

    setBusy(true)
    setMessage('')
    try {
      await deleteAgent(selectedAgentID)
      const next = agents.filter((item) => item.id !== selectedAgentID)
      setAgents(next)
      if (next.length > 0) {
        setSelectedAgentID(next[0].id)
        setAgentDraft(next[0])
      } else {
        startNewAgent()
      }
      setMessage('Agent deleted.')
    } catch (error) {
      setMessage(error instanceof Error ? error.message : 'Failed to delete agent')
    } finally {
      setBusy(false)
    }
  }

  async function savePlaybook() {
    setBusy(true)
    setMessage('')
    try {
      let phases = playbookDraft.phases
      if (phaseEditorMode === 'raw') {
        const parsed = parseRawPhases(phasesEditor)
        if (parsed.error) {
          setMessage(parsed.error)
          return
        }
        phases = parsed.phases
      }
      const payload: Playbook = {
        ...playbookDraft,
        phases,
      }
      if (isNewPlaybook) {
        const created = await createPlaybook<Playbook>(payload)
        setPlaybooks((prev) => [...prev.filter((item) => item.id !== created.id), created].sort((a, b) => a.name.localeCompare(b.name)))
        setSelectedPlaybookID(created.id)
        setPlaybookDraft(created)
        setDraftPhases(created.phases)
      } else {
        const updated = await updatePlaybook<Playbook>(selectedPlaybookID, payload)
        setPlaybooks((prev) => prev.map((item) => (item.id === updated.id ? updated : item)))
        setPlaybookDraft(updated)
        setDraftPhases(updated.phases)
      }
      setMessage('Playbook saved.')
    } catch (error) {
      if (error instanceof SyntaxError) {
        setMessage('Phases must be valid JSON array.')
      } else {
        setMessage(error instanceof Error ? error.message : 'Failed to save playbook')
      }
    } finally {
      setBusy(false)
    }
  }

  async function removePlaybook() {
    if (!selectedPlaybookID || !selectedPlaybook) {
      return
    }
    if (!window.confirm(`Delete playbook ${selectedPlaybook.name}?`)) {
      return
    }

    setBusy(true)
    setMessage('')
    try {
      await deletePlaybook(selectedPlaybookID)
      const next = playbooks.filter((item) => item.id !== selectedPlaybookID)
      setPlaybooks(next)
      if (next.length > 0) {
        setSelectedPlaybookID(next[0].id)
        setPlaybookDraft(next[0])
        setPhasesEditor(stringifyPhases(next[0].phases))
      } else {
        startNewPlaybook()
      }
      setMessage('Playbook deleted.')
    } catch (error) {
      setMessage(error instanceof Error ? error.message : 'Failed to delete playbook')
    } finally {
      setBusy(false)
    }
  }

  if (loading) {
    return (
      <section className="page-block settings-page">
        <h2>Settings</h2>
        <p>Loading configuration...</p>
      </section>
    )
  }

  return (
    <section className="page-block settings-page">
      <h2>Settings</h2>
      <div className="settings-tabs">
        <button type="button" className={`secondary-btn ${activeTab === 'agents' ? 'active' : ''}`} onClick={() => setActiveTab('agents')}>
          Agent Registry
        </button>
        <button type="button" className={`secondary-btn ${activeTab === 'playbooks' ? 'active' : ''}`} onClick={() => setActiveTab('playbooks')}>
          Playbooks
        </button>
      </div>

      {message ? <p className="settings-message">{message}</p> : null}

      {activeTab === 'agents' ? (
        <div className="settings-grid">
          <aside className="settings-list">
            <button type="button" className="primary-btn" onClick={startNewAgent}>
              + New Agent
            </button>
            {agents.map((item) => (
              <button
                key={item.id}
                type="button"
                className={`session-row ${item.id === selectedAgentID ? 'active' : ''}`}
                onClick={() => selectAgent(item.id)}
              >
                <strong>{item.name}</strong>
                <small>{item.id}</small>
              </button>
            ))}
          </aside>

          <div className="settings-editor">
            <label>
              ID
              <input
                value={agentDraft.id}
                disabled={!isNewAgent}
                onChange={(event) => setAgentDraft((prev) => ({ ...prev, id: event.target.value.trim().toLowerCase() }))}
              />
            </label>
            <label>
              Name
              <input value={agentDraft.name} onChange={(event) => setAgentDraft((prev) => ({ ...prev, name: event.target.value }))} />
            </label>
            <label>
              Command
              <input value={agentDraft.command} onChange={(event) => setAgentDraft((prev) => ({ ...prev, command: event.target.value }))} />
            </label>
            <label>
              Model
              <input value={agentDraft.model ?? ''} onChange={(event) => setAgentDraft((prev) => ({ ...prev, model: event.target.value }))} />
            </label>
            <label>
              Languages (comma-separated)
              <input
                value={(agentDraft.languages ?? []).join(', ')}
                onChange={(event) => setAgentDraft((prev) => ({ ...prev, languages: parseCSV(event.target.value) }))}
              />
            </label>
            <label>
              Capabilities (comma-separated)
              <input
                value={(agentDraft.capabilities ?? []).join(', ')}
                onChange={(event) => setAgentDraft((prev) => ({ ...prev, capabilities: parseCSV(event.target.value) }))}
              />
            </label>
            <div className="settings-actions">
              <button type="button" className="primary-btn" disabled={busy} onClick={() => void saveAgent()}>
                Save Agent
              </button>
              {!isNewAgent ? (
                <button type="button" className="action-btn danger" disabled={busy} onClick={() => void removeAgent()}>
                  Delete Agent
                </button>
              ) : null}
            </div>
          </div>
        </div>
      ) : (
        <div className="settings-grid">
          <aside className="settings-list">
            <button type="button" className="primary-btn" onClick={startNewPlaybook}>
              + New Playbook
            </button>
            {playbooks.map((item) => (
              <button
                key={item.id}
                type="button"
                className={`session-row ${item.id === selectedPlaybookID ? 'active' : ''}`}
                onClick={() => selectPlaybook(item.id)}
              >
                <strong>{item.name}</strong>
                <small>{item.id}</small>
              </button>
            ))}
          </aside>

          <div className="settings-editor">
            <label>
              ID
              <input
                value={playbookDraft.id}
                disabled={!isNewPlaybook}
                onChange={(event) => setPlaybookDraft((prev) => ({ ...prev, id: event.target.value.trim().toLowerCase() }))}
              />
            </label>
            <label>
              Name
              <input value={playbookDraft.name} onChange={(event) => setPlaybookDraft((prev) => ({ ...prev, name: event.target.value }))} />
            </label>
            <label>
              Description
              <input value={playbookDraft.description} onChange={(event) => setPlaybookDraft((prev) => ({ ...prev, description: event.target.value }))} />
            </label>
            <label>
              Parallelism Strategy
              <input
                value={playbookDraft.parallelism_strategy}
                onChange={(event) => setPlaybookDraft((prev) => ({ ...prev, parallelism_strategy: event.target.value }))}
              />
            </label>
            <label>
              Match Languages (comma-separated)
              <input
                value={playbookDraft.match.languages.join(', ')}
                onChange={(event) =>
                  setPlaybookDraft((prev) => ({
                    ...prev,
                    match: { ...prev.match, languages: parseCSV(event.target.value) },
                  }))
                }
              />
            </label>
            <label>
              Match Project Patterns (comma-separated)
              <input
                value={playbookDraft.match.project_patterns.join(', ')}
                onChange={(event) =>
                  setPlaybookDraft((prev) => ({
                    ...prev,
                    match: { ...prev.match, project_patterns: parseCSV(event.target.value) },
                  }))
                }
              />
            </label>
            <div className="settings-phase-editor">
              <div className="settings-phase-editor-head">
                <span>Phases</span>
                <div className="settings-phase-editor-modes">
                  <button
                    type="button"
                    className={`secondary-btn ${phaseEditorMode === 'table' ? 'active' : ''}`}
                    onClick={() => switchPhaseEditorMode('table')}
                  >
                    Table
                  </button>
                  <button
                    type="button"
                    className={`secondary-btn ${phaseEditorMode === 'raw' ? 'active' : ''}`}
                    onClick={() => switchPhaseEditorMode('raw')}
                  >
                    Raw JSON
                  </button>
                </div>
              </div>
              {phaseEditorMode === 'table' ? (
                <>
                  <table className="settings-phase-table">
                    <thead>
                      <tr>
                        <th>Name</th>
                        <th>Agent</th>
                        <th>Role</th>
                        <th>Description</th>
                        <th />
                      </tr>
                    </thead>
                    <tbody>
                      {playbookDraft.phases.map((phase, index) => (
                        <tr key={`${index}-${phase.name}-${phase.agent}`}>
                          <td>
                            <input value={phase.name} onChange={(event) => updatePhase(index, { name: event.target.value })} />
                          </td>
                          <td>
                            <input value={phase.agent} onChange={(event) => updatePhase(index, { agent: event.target.value })} />
                          </td>
                          <td>
                            <input value={phase.role} onChange={(event) => updatePhase(index, { role: event.target.value })} />
                          </td>
                          <td>
                            <input value={phase.description} onChange={(event) => updatePhase(index, { description: event.target.value })} />
                          </td>
                          <td>
                            <button type="button" className="action-btn danger" onClick={() => removePhase(index)}>
                              Remove
                            </button>
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                  <button type="button" className="secondary-btn settings-phase-add" onClick={addPhase}>
                    + Add Phase
                  </button>
                </>
              ) : (
                <textarea className="settings-code" value={phasesEditor} onChange={(event) => setPhasesEditor(event.target.value)} />
              )}
            </div>
            <div className="settings-actions">
              <button type="button" className="primary-btn" disabled={busy} onClick={() => void savePlaybook()}>
                Save Playbook
              </button>
              {!isNewPlaybook ? (
                <button type="button" className="action-btn danger" disabled={busy} onClick={() => void removePlaybook()}>
                  Delete Playbook
                </button>
              ) : null}
            </div>
          </div>
        </div>
      )}
    </section>
  )
}
