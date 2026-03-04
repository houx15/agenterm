import { useState } from 'react'
import { createProject } from '../api/client'
import type { Project } from '../api/types'
import { useAppContext } from '../App'

interface NewProjectModalProps {
  open: boolean
  onClose: () => void
  onCreated: () => void
}

export default function NewProjectModal({ open, onClose, onCreated }: NewProjectModalProps) {
  const { setSelectedProjectID } = useAppContext()
  const [name, setName] = useState('')
  const [repoPath, setRepoPath] = useState('')
  const [creating, setCreating] = useState(false)
  const [error, setError] = useState('')

  if (!open) return null

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!name.trim() || !repoPath.trim()) return

    setCreating(true)
    setError('')
    try {
      const project = await createProject<Project>({
        name: name.trim(),
        repo_path: repoPath.trim(),
      })
      setSelectedProjectID(project.id)
      setName('')
      setRepoPath('')
      onCreated()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create project')
    } finally {
      setCreating(false)
    }
  }

  const handleBackdropClick = (e: React.MouseEvent) => {
    if (e.target === e.currentTarget) {
      onClose()
    }
  }

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/60"
      onClick={handleBackdropClick}
      role="presentation"
    >
      <div
        className="w-full max-w-md rounded-lg border border-border bg-bg-secondary p-6 shadow-xl"
        role="dialog"
        aria-modal="true"
      >
        <h2 className="text-lg font-semibold text-text-primary mb-4">New Project</h2>

        <form onSubmit={(e) => void handleSubmit(e)} className="flex flex-col gap-4">
          <div className="flex flex-col gap-1.5">
            <label htmlFor="project-name" className="text-sm text-text-secondary">
              Project Name
            </label>
            <input
              id="project-name"
              type="text"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="My Project"
              className="rounded border border-border bg-bg-tertiary px-3 py-2 text-sm text-text-primary placeholder:text-text-secondary/50 focus:border-accent focus:outline-none"
              autoFocus
            />
          </div>

          <div className="flex flex-col gap-1.5">
            <label htmlFor="repo-path" className="text-sm text-text-secondary">
              Folder Path
            </label>
            <input
              id="repo-path"
              type="text"
              value={repoPath}
              onChange={(e) => setRepoPath(e.target.value)}
              placeholder="/path/to/your/project"
              className="rounded border border-border bg-bg-tertiary px-3 py-2 text-sm text-text-primary placeholder:text-text-secondary/50 focus:border-accent focus:outline-none"
            />
          </div>

          {error && (
            <p className="text-sm text-status-error">{error}</p>
          )}

          <div className="flex justify-end gap-3 pt-2">
            <button
              type="button"
              onClick={onClose}
              disabled={creating}
              className="rounded px-4 py-2 text-sm text-text-secondary hover:text-text-primary hover:bg-bg-tertiary transition-colors"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={creating || !name.trim() || !repoPath.trim()}
              className="rounded bg-accent px-4 py-2 text-sm font-medium text-white hover:bg-accent/80 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
            >
              {creating ? 'Creating...' : 'Create'}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}
