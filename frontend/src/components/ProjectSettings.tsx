import { useCallback, useEffect, useState } from 'react'
import { getProject, updateProject } from '../api/client'
import type { Project } from '../api/types'

interface ProjectSettingsProps {
  projectID: string
}

interface ProjectData extends Project {
  context_template?: string
  knowledge_summary?: string
}

export default function ProjectSettings({ projectID }: ProjectSettingsProps) {
  const [project, setProject] = useState<ProjectData | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  // Editable fields
  const [name, setName] = useState('')
  const [contextTemplate, setContextTemplate] = useState('')

  // Save states
  const [savingName, setSavingName] = useState(false)
  const [savingTemplate, setSavingTemplate] = useState(false)
  const [notice, setNotice] = useState<{ kind: 'success' | 'error'; text: string } | null>(null)

  const fetchProject = useCallback(async () => {
    setLoading(true)
    setError('')
    try {
      const data = await getProject<ProjectData>(projectID)
      setProject(data)
      setName(data.name)
      setContextTemplate(data.context_template ?? '')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load project')
    } finally {
      setLoading(false)
    }
  }, [projectID])

  useEffect(() => {
    void fetchProject()
  }, [fetchProject])

  const saveName = async () => {
    const trimmed = name.trim()
    if (!trimmed || trimmed === project?.name) return
    setSavingName(true)
    setNotice(null)
    try {
      const updated = await updateProject<ProjectData>(projectID, { name: trimmed })
      setProject(updated)
      setName(updated.name)
      setNotice({ kind: 'success', text: 'Project name updated.' })
    } catch (err) {
      setNotice({ kind: 'error', text: err instanceof Error ? err.message : 'Failed to save name' })
    } finally {
      setSavingName(false)
    }
  }

  const saveContextTemplate = async () => {
    setSavingTemplate(true)
    setNotice(null)
    try {
      const updated = await updateProject<ProjectData>(projectID, { context_template: contextTemplate })
      setProject(updated)
      setNotice({ kind: 'success', text: 'Context template saved.' })
    } catch (err) {
      setNotice({ kind: 'error', text: err instanceof Error ? err.message : 'Failed to save template' })
    } finally {
      setSavingTemplate(false)
    }
  }

  const handleNameKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter') {
      void saveName()
    }
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center h-full text-text-secondary text-sm">
        Loading project settings...
      </div>
    )
  }

  if (error) {
    return (
      <div className="flex items-center justify-center h-full text-status-error text-sm">
        {error}
      </div>
    )
  }

  if (!project) return null

  return (
    <div className="h-full overflow-y-auto p-6">
      <div className="max-w-2xl mx-auto space-y-8">
        <h2 className="text-lg font-semibold text-text-primary">Project Settings</h2>

        {/* Notice */}
        {notice && (
          <div
            className={`rounded border px-4 py-2 text-sm ${
              notice.kind === 'success'
                ? 'border-status-working/50 bg-status-working/10 text-status-working'
                : 'border-status-error/50 bg-status-error/10 text-status-error'
            }`}
          >
            {notice.text}
          </div>
        )}

        {/* Project Name */}
        <div>
          <label className="block text-sm font-medium text-text-secondary mb-2">
            Project Name
          </label>
          <input
            value={name}
            onChange={(e) => setName(e.target.value)}
            onBlur={() => void saveName()}
            onKeyDown={handleNameKeyDown}
            disabled={savingName}
            className="w-full rounded border border-border bg-bg-tertiary px-4 py-2.5 text-sm text-text-primary focus:border-accent focus:outline-none disabled:opacity-50"
            placeholder="Project name"
          />
          <p className="mt-1 text-xs text-text-secondary">
            Press Enter or click away to save.
          </p>
        </div>

        {/* Context Template */}
        <div>
          <label className="block text-sm font-medium text-text-secondary mb-2">
            Context Template
          </label>
          <p className="text-xs text-text-secondary mb-2">
            This text gets injected into every worktree context file (e.g. CLAUDE.md / AGENTS.md).
          </p>
          <textarea
            value={contextTemplate}
            onChange={(e) => setContextTemplate(e.target.value)}
            disabled={savingTemplate}
            rows={14}
            className="w-full rounded border border-border bg-bg-tertiary px-4 py-3 text-sm text-text-primary font-mono leading-relaxed focus:border-accent focus:outline-none resize-y disabled:opacity-50"
            placeholder="# Project Context&#10;&#10;Describe project conventions, architecture, and instructions for agents..."
          />
          <div className="mt-2">
            <button
              onClick={() => void saveContextTemplate()}
              disabled={savingTemplate}
              className="rounded bg-accent px-5 py-2 text-sm font-medium text-white hover:bg-accent/80 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
            >
              {savingTemplate ? 'Saving...' : 'Save Template'}
            </button>
          </div>
        </div>

        {/* Project Knowledge */}
        <div>
          <label className="block text-sm font-medium text-text-secondary mb-2">
            Project Knowledge
          </label>
          <p className="text-xs text-text-secondary mb-2">
            Auto-generated summary from project initialization. Read-only.
          </p>
          {project.knowledge_summary ? (
            <div className="rounded border border-border bg-bg-tertiary px-4 py-3 text-sm text-text-primary font-mono whitespace-pre-wrap leading-relaxed">
              {project.knowledge_summary}
            </div>
          ) : (
            <div className="rounded border border-border bg-bg-primary/50 px-4 py-6 text-center text-sm text-text-secondary">
              No project knowledge yet. Run project initialization to generate a knowledge summary.
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
