import type { Project } from '../api/types'
import { FolderOpen, Plus } from './Lucide'

interface HomeViewProps {
  projects: Project[]
  onSelectProject: (id: string) => void
  onCreateProject: () => void
}

function formatDate(iso: string): string {
  try {
    const d = new Date(iso)
    return d.toLocaleDateString(undefined, { month: 'short', day: 'numeric', year: 'numeric' })
  } catch {
    return iso
  }
}

function statusBadgeColor(status: string): string {
  switch (status) {
    case 'active':
      return 'var(--status-green, #22c55e)'
    case 'paused':
      return 'var(--status-yellow, #eab308)'
    case 'error':
      return 'var(--status-red, #ef4444)'
    default:
      return 'var(--text-tertiary, #888)'
  }
}

export default function HomeView({ projects, onSelectProject, onCreateProject }: HomeViewProps) {
  if (projects.length === 0) {
    return (
      <div className="home-view">
        <h1 className="home-brand home-view-title">agenTerm</h1>
        <div className="empty-state">
          <p>No projects yet. Create your first project to get started.</p>
          <button className="btn btn-primary" onClick={onCreateProject}>
            <Plus size={14} />
            Create your first project
          </button>
        </div>
      </div>
    )
  }

  return (
    <div className="home-view">
      <h1 className="home-brand home-view-title">agenTerm</h1>

      <div className="home-stats">
        <span className="home-stat">
          {projects.length} project{projects.length !== 1 ? 's' : ''}
        </span>
      </div>

      <div className="home-projects">
        {projects.map((project) => (
          <div
            key={project.id}
            className="home-project-card"
            onClick={() => onSelectProject(project.id)}
            role="button"
            tabIndex={0}
            onKeyDown={(e) => {
              if (e.key === 'Enter' || e.key === ' ') {
                e.preventDefault()
                onSelectProject(project.id)
              }
            }}
          >
            <h4 style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
              <FolderOpen size={14} />
              {project.name}
            </h4>
            <p>{project.repo_path}</p>
            <p style={{ display: 'flex', alignItems: 'center', gap: '8px', marginTop: '6px' }}>
              <span
                style={{
                  display: 'inline-block',
                  width: 8,
                  height: 8,
                  borderRadius: '50%',
                  background: statusBadgeColor(project.status),
                }}
              />
              {project.status}
              <span style={{ marginLeft: 'auto' }}>{formatDate(project.created_at)}</span>
            </p>
          </div>
        ))}
      </div>

      <button className="btn btn-primary" onClick={onCreateProject} style={{ marginTop: '8px' }}>
        <Plus size={14} />
        Create Project
      </button>
    </div>
  )
}
