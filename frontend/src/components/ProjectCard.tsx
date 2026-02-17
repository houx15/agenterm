import type { Project } from '../api/types'

interface ProjectCardProps {
  project?: Project
  doneTasks?: number
  totalTasks?: number
  activeAgents?: number
  onClick: () => void
  isNewCard?: boolean
}

export default function ProjectCard({
  project,
  doneTasks = 0,
  totalTasks = 0,
  activeAgents = 0,
  onClick,
  isNewCard = false,
}: ProjectCardProps) {
  if (isNewCard) {
    return (
      <button className="project-card project-card-new" onClick={onClick} type="button">
        <strong>+ New Project</strong>
        <p>Create and start tracking a project</p>
      </button>
    )
  }

  if (!project) {
    return null
  }

  return (
    <button className="project-card" onClick={onClick} type="button">
      <div className="project-card-head">
        <strong>{project.name}</strong>
        <small>{project.status || 'active'}</small>
      </div>
      <p>{doneTasks}/{totalTasks} done</p>
      <p>{activeAgents} active agents</p>
    </button>
  )
}
