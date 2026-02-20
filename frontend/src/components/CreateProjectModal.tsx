import { useEffect, useMemo, useState } from 'react'
import { listDirectories } from '../api/client'
import type { AgentConfig, Playbook } from '../api/types'
import Modal from './Modal'

interface CreateProjectValues {
  name: string
  repoPath: string
  orchestratorAgentID: string
  orchestratorProvider: string
  playbook: string
  workers: number
}

interface CreateProjectModalProps {
  open: boolean
  playbooks: Playbook[]
  agents: AgentConfig[]
  busy?: boolean
  onClose: () => void
  onSubmit: (values: CreateProjectValues) => Promise<void> | void
}

interface BrowseState {
  path: string
  parent?: string
  directories: Array<{ name: string; path: string }>
}

export default function CreateProjectModal({
  open,
  playbooks,
  agents,
  busy = false,
  onClose,
  onSubmit,
}: CreateProjectModalProps) {
  const [name, setName] = useState('')
  const [repoPath, setRepoPath] = useState('')
  const [orchestratorAgentID, setOrchestratorAgentID] = useState('')
  const [playbook, setPlaybook] = useState('')
  const [workers, setWorkers] = useState(2)
  const [error, setError] = useState('')
  const [browseOpen, setBrowseOpen] = useState(false)
  const [browsePathInput, setBrowsePathInput] = useState('')
  const [browseState, setBrowseState] = useState<BrowseState | null>(null)
  const [browseBusy, setBrowseBusy] = useState(false)
  const [browseError, setBrowseError] = useState('')
  const orchestratorAgents = useMemo(() => agents.filter((item) => item.supports_orchestrator), [agents])

  const defaultPlaybook = useMemo(() => playbooks[0]?.id ?? '', [playbooks])
  const defaultOrchestratorAgentID = useMemo(() => {
    const preferred = orchestratorAgents.find((item) => item.id === 'orchestrator')
    return preferred?.id ?? orchestratorAgents[0]?.id ?? ''
  }, [orchestratorAgents])

  useEffect(() => {
    if (!open) {
      return
    }
    setName('')
    setRepoPath('')
    setError('')
    setPlaybook(defaultPlaybook)
    setOrchestratorAgentID(defaultOrchestratorAgentID)
    setWorkers(2)
    setBrowseOpen(false)
    setBrowsePathInput('')
    setBrowseState(null)
    setBrowseBusy(false)
    setBrowseError('')
  }, [defaultOrchestratorAgentID, defaultPlaybook, open, orchestratorAgents])

  const loadBrowsePath = async (path?: string) => {
    setBrowseBusy(true)
    setBrowseError('')
    try {
      const next = await listDirectories(path)
      setBrowseState(next)
      setBrowsePathInput(next.path)
    } catch (err) {
      setBrowseError(err instanceof Error ? err.message : String(err))
    } finally {
      setBrowseBusy(false)
    }
  }

  const openBrowse = async () => {
    setBrowseOpen(true)
    await loadBrowsePath(repoPath.trim() || undefined)
  }

  const selectDirectory = (path: string) => {
    setRepoPath(path)
    if (!name) {
      const segments = path.split(/[\\/]/).filter(Boolean)
      if (segments.length > 0) {
        setName(segments[segments.length - 1])
      }
    }
    setBrowseOpen(false)
    setBrowseError('')
  }

  const submit = async () => {
    const trimmedName = name.trim()
    const trimmedRepoPath = repoPath.trim()
    if (!trimmedName || !trimmedRepoPath) {
      setError('Project name and folder are required.')
      return
    }
    setError('')
    await onSubmit({
      name: trimmedName,
      repoPath: trimmedRepoPath,
      orchestratorAgentID,
      orchestratorProvider: orchestratorAgents.find((item) => item.id === orchestratorAgentID)?.orchestrator_provider || 'anthropic',
      playbook,
      workers,
    })
  }

  return (
    <Modal onClose={onClose} open={open} title="Create Project">
      <div className="modal-form">
        <label className="settings-field">
          <span>Project Name</span>
          <input onChange={(event) => setName(event.target.value)} placeholder="Agenterm iOS app" value={name} />
        </label>

        <label className="settings-field">
          <span>Project Folder</span>
          <div className="inline-field-row">
            <input
              onChange={(event) => setRepoPath(event.target.value)}
              placeholder="/Users/you/code/my-project"
              value={repoPath}
            />
            <button className="secondary-btn" onClick={() => void openBrowse()} type="button">
              Browse
            </button>
          </div>
        </label>
        {browseOpen && (
          <div className="folder-browser">
            <div className="folder-browser-path-row">
              <input
                onChange={(event) => setBrowsePathInput(event.target.value)}
                placeholder="/Users/you"
                value={browsePathInput}
              />
              <button
                className="secondary-btn"
                disabled={browseBusy}
                onClick={() => void loadBrowsePath(browsePathInput)}
                type="button"
              >
                Open
              </button>
              <button
                className="secondary-btn"
                disabled={browseBusy || !browseState?.parent}
                onClick={() => void loadBrowsePath(browseState?.parent)}
                type="button"
              >
                Up
              </button>
            </div>
            {browseBusy ? (
              <p className="folder-browser-hint">Loading folders...</p>
            ) : browseState ? (
              <>
                <p className="folder-browser-hint">Current: {browseState.path}</p>
                <div className="folder-browser-actions">
                  <button className="secondary-btn" onClick={() => selectDirectory(browseState.path)} type="button">
                    Use This Folder
                  </button>
                </div>
                <div className="folder-browser-list">
                  {browseState.directories.map((dir) => (
                    <button
                      className="folder-browser-item"
                      key={dir.path}
                      onClick={() => void loadBrowsePath(dir.path)}
                      type="button"
                    >
                      {dir.name}
                    </button>
                  ))}
                  {browseState.directories.length === 0 && <p className="folder-browser-hint">No subfolders</p>}
                </div>
              </>
            ) : null}
            {browseError && <p className="dashboard-error">{browseError}</p>}
          </div>
        )}

        <label className="settings-field">
          <span>Orchestrator Agent</span>
          <select
            onChange={(event) => {
              const nextID = event.target.value
              setOrchestratorAgentID(nextID)
            }}
            value={orchestratorAgentID}
          >
            {orchestratorAgents.length === 0 && <option value="">No orchestrator-capable agents</option>}
            {orchestratorAgents.map((agent) => (
              <option key={agent.id} value={agent.id}>
                {agent.name} ({agent.id})
              </option>
            ))}
          </select>
        </label>

        <label className="settings-field">
          <span>Playbook (Workflow)</span>
          <select onChange={(event) => setPlaybook(event.target.value)} value={playbook}>
            <option value="">Default auto-match</option>
            {playbooks.map((item) => (
              <option key={item.id} value={item.id}>
                {item.name}
              </option>
            ))}
          </select>
        </label>

        <label className="settings-field">
          <span>Assigned Workers</span>
          <input
            max={64}
            min={1}
            onChange={(event) => setWorkers(Math.max(1, Math.min(64, Number(event.target.value) || 1)))}
            type="number"
            value={workers}
          />
        </label>

        {error && <p className="dashboard-error">{error}</p>}

        <div className="settings-actions">
          <button className="secondary-btn" disabled={busy} onClick={onClose} type="button">
            Cancel
          </button>
          <button className="primary-btn" disabled={busy} onClick={() => void submit()} type="button">
            {busy ? 'Creating...' : 'Create Project'}
          </button>
        </div>
      </div>
    </Modal>
  )
}
