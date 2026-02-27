import { useEffect, useMemo, useState } from 'react'
import {
  createProject,
  listAgents,
  listDirectories,
  updateProjectOrchestrator,
} from '../api/client'
import type { AgentConfig, Project, ProjectOrchestratorProfile } from '../api/types'
import { FolderOpen } from './Lucide'
import Modal from './Modal'

interface CreateProjectFlowProps {
  open: boolean
  onClose: () => void
  onCreated: () => void
}

interface BrowseState {
  path: string
  parent?: string
  directories: Array<{ name: string; path: string }>
}

export default function CreateProjectFlow({ open, onClose, onCreated }: CreateProjectFlowProps) {
  const [agents, setAgents] = useState<AgentConfig[]>([])
  const [name, setName] = useState('')
  const [repoPath, setRepoPath] = useState('')
  const [orchestratorAgentID, setOrchestratorAgentID] = useState('')
  const [workers, setWorkers] = useState(2)
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  // Directory browser state
  const [browseOpen, setBrowseOpen] = useState(false)
  const [browsePathInput, setBrowsePathInput] = useState('')
  const [browseState, setBrowseState] = useState<BrowseState | null>(null)
  const [browseBusy, setBrowseBusy] = useState(false)
  const [browseError, setBrowseError] = useState('')

  const orchestratorAgents = useMemo(
    () => agents.filter((a) => a.supports_orchestrator),
    [agents],
  )

  const defaultOrchestratorAgentID = useMemo(() => {
    const preferred = orchestratorAgents.find((a) => a.id === 'orchestrator')
    return preferred?.id ?? orchestratorAgents[0]?.id ?? ''
  }, [orchestratorAgents])

  // Load agents on mount
  useEffect(() => {
    if (!open) return
    void listAgents<AgentConfig[]>().then(setAgents).catch(() => {})
  }, [open])

  // Reset form when modal opens
  useEffect(() => {
    if (!open) return
    setName('')
    setRepoPath('')
    setError('')
    setWorkers(2)
    setBusy(false)
    setBrowseOpen(false)
    setBrowsePathInput('')
    setBrowseState(null)
    setBrowseBusy(false)
    setBrowseError('')
  }, [open])

  // Sync default agent selection when agents load
  useEffect(() => {
    if (defaultOrchestratorAgentID) {
      setOrchestratorAgentID(defaultOrchestratorAgentID)
    }
  }, [defaultOrchestratorAgentID])

  const loadBrowsePath = async (path?: string) => {
    setBrowseBusy(true)
    setBrowseError('')
    try {
      const result = await listDirectories(path)
      setBrowseState(result)
      setBrowsePathInput(result.path)
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
    if (!orchestratorAgentID) {
      setError('Please select an orchestrator agent.')
      return
    }
    setError('')
    setBusy(true)
    try {
      const selectedAgent = orchestratorAgents.find((a) => a.id === orchestratorAgentID)
      const created = await createProject<Project>({
        name: trimmedName,
        repo_path: trimmedRepoPath,
        status: 'active',
      })
      await updateProjectOrchestrator<ProjectOrchestratorProfile>(created.id, {
        default_provider: selectedAgent?.orchestrator_provider || 'anthropic',
        default_model: selectedAgent?.model || '',
        max_parallel: workers,
      })
      onCreated()
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setBusy(false)
    }
  }

  return (
    <Modal onClose={onClose} open={open} title="Create Project">
      <div className="modal-form">
        <label className="settings-field">
          <span>Project Name</span>
          <input
            onChange={(e) => setName(e.target.value)}
            placeholder="My Awesome Project"
            value={name}
          />
        </label>

        <label className="settings-field">
          <span>Project Folder</span>
          <div className="inline-field-row">
            <input
              onChange={(e) => setRepoPath(e.target.value)}
              placeholder="/Users/you/code/my-project"
              value={repoPath}
            />
            <button
              className="btn btn-ghost"
              onClick={() => void openBrowse()}
              type="button"
            >
              <FolderOpen size={14} /> Browse
            </button>
          </div>
        </label>

        {browseOpen && (
          <div className="folder-browser">
            <div className="folder-browser-path-row">
              <input
                onChange={(e) => setBrowsePathInput(e.target.value)}
                placeholder="/Users/you"
                value={browsePathInput}
              />
              <button
                className="btn btn-ghost"
                disabled={browseBusy}
                onClick={() => void loadBrowsePath(browsePathInput)}
                type="button"
              >
                Open
              </button>
              <button
                className="btn btn-ghost"
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
                  <button
                    className="btn btn-primary"
                    onClick={() => selectDirectory(browseState.path)}
                    type="button"
                  >
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
                  {browseState.directories.length === 0 && (
                    <p className="folder-browser-hint">No subfolders</p>
                  )}
                </div>
              </>
            ) : null}
            {browseError && <p className="dashboard-error">{browseError}</p>}
          </div>
        )}

        <label className="settings-field">
          <span>Orchestrator Agent</span>
          <select
            onChange={(e) => setOrchestratorAgentID(e.target.value)}
            value={orchestratorAgentID}
          >
            {orchestratorAgents.length === 0 && (
              <option value="">No orchestrator-capable agents</option>
            )}
            {orchestratorAgents.map((agent) => (
              <option key={agent.id} value={agent.id}>
                {agent.name} ({agent.id})
              </option>
            ))}
          </select>
        </label>

        <label className="settings-field">
          <span>Assigned Workers</span>
          <input
            max={64}
            min={1}
            onChange={(e) => setWorkers(Math.max(1, Math.min(64, Number(e.target.value) || 1)))}
            type="number"
            value={workers}
          />
        </label>

        {error && <p className="dashboard-error">{error}</p>}

        <div className="settings-actions">
          <button className="btn btn-ghost" disabled={busy} onClick={onClose} type="button">
            Cancel
          </button>
          <button
            className="btn btn-primary"
            disabled={busy}
            onClick={() => void submit()}
            type="button"
          >
            {busy ? 'Creating...' : 'Create Project'}
          </button>
        </div>
      </div>
    </Modal>
  )
}
