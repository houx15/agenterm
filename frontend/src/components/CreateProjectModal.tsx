import { useEffect, useMemo, useRef, useState } from 'react'
import type { AgentConfig, Playbook } from '../api/types'
import Modal from './Modal'

interface CreateProjectValues {
  name: string
  repoPath: string
  orchestratorAgentID: string
  playbook: string
  orchestratorModel: string
  workers: number
}

interface CreateProjectModalProps {
  open: boolean
  playbooks: Playbook[]
  agents: AgentConfig[]
  modelOptions: string[]
  busy?: boolean
  onClose: () => void
  onSubmit: (values: CreateProjectValues) => Promise<void> | void
}

interface DirectoryPickerWindow extends Window {
  showDirectoryPicker?: () => Promise<{ name?: string }>
}

export default function CreateProjectModal({
  open,
  playbooks,
  agents,
  modelOptions,
  busy = false,
  onClose,
  onSubmit,
}: CreateProjectModalProps) {
  const folderInputRef = useRef<HTMLInputElement | null>(null)
  const [name, setName] = useState('')
  const [repoPath, setRepoPath] = useState('')
  const [orchestratorAgentID, setOrchestratorAgentID] = useState('')
  const [orchestratorModel, setOrchestratorModel] = useState('')
  const [playbook, setPlaybook] = useState('')
  const [workers, setWorkers] = useState(2)
  const [error, setError] = useState('')

  const defaultPlaybook = useMemo(() => playbooks[0]?.id ?? '', [playbooks])
  const defaultModel = useMemo(() => modelOptions[0] ?? '', [modelOptions])
  const defaultOrchestratorAgentID = useMemo(() => {
    const preferred = agents.find((item) => item.id === 'orchestrator')
    return preferred?.id ?? agents[0]?.id ?? ''
  }, [agents])

  useEffect(() => {
    if (!open) {
      return
    }
    setName('')
    setRepoPath('')
    setError('')
    setPlaybook(defaultPlaybook)
    setOrchestratorAgentID(defaultOrchestratorAgentID)
    const selected = agents.find((item) => item.id === defaultOrchestratorAgentID)
    setOrchestratorModel((selected?.model || '').trim() || defaultModel)
    setWorkers(2)
  }, [agents, defaultModel, defaultOrchestratorAgentID, defaultPlaybook, open])

  useEffect(() => {
    if (!folderInputRef.current) {
      return
    }
    const input = folderInputRef.current as HTMLInputElement & {
      webkitdirectory?: boolean
      directory?: boolean
    }
    input.webkitdirectory = true
    input.directory = true
  }, [])

  const pickFolder = async () => {
    const pickerWindow = window as DirectoryPickerWindow
    if (pickerWindow.showDirectoryPicker) {
      try {
        const handle = await pickerWindow.showDirectoryPicker()
        if (handle?.name) {
          setRepoPath((prev) => prev || handle.name || '')
          if (!name) {
            setName(handle.name || '')
          }
        }
        return
      } catch {
        // fall back to input picker
      }
    }
    folderInputRef.current?.click()
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
      playbook,
      orchestratorModel: orchestratorModel.trim(),
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
            <button className="secondary-btn" onClick={() => void pickFolder()} type="button">
              Browse
            </button>
          </div>
          <input
            className="sr-only"
            onChange={(event) => {
              const file = event.target.files?.[0] as (File & { path?: string }) | undefined
              const fallbackName = file?.webkitRelativePath?.split('/')[0] ?? ''
              const picked = (file?.path || '').trim()
              if (picked) {
                setRepoPath(picked)
                if (!name) {
                  const segments = picked.split(/[\\/]/).filter(Boolean)
                  setName(segments[segments.length - 1] ?? '')
                }
              } else if (fallbackName) {
                setRepoPath((prev) => prev || fallbackName)
                if (!name) {
                  setName(fallbackName)
                }
              }
            }}
            ref={folderInputRef}
            type="file"
          />
        </label>

        <label className="settings-field">
          <span>Orchestrator Agent</span>
          <select
            onChange={(event) => {
              const nextID = event.target.value
              setOrchestratorAgentID(nextID)
              const selected = agents.find((item) => item.id === nextID)
              if (selected?.model) {
                setOrchestratorModel(selected.model)
              }
            }}
            value={orchestratorAgentID}
          >
            {agents.length === 0 && <option value="">No agents in registry</option>}
            {agents.map((agent) => (
              <option key={agent.id} value={agent.id}>
                {agent.name} ({agent.id})
              </option>
            ))}
          </select>
        </label>

        <label className="settings-field">
          <span>Orchestrator Model</span>
          <input
            list="orchestrator-models"
            onChange={(event) => setOrchestratorModel(event.target.value)}
            placeholder="gpt-5-codex"
            value={orchestratorModel}
          />
          <datalist id="orchestrator-models">
            {modelOptions.map((option) => (
              <option key={option} value={option} />
            ))}
          </datalist>
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
