import type { Project } from '../api/types'

interface ProjectSelectorProps {
  projects: Project[]
  value: string
  onChange: (projectID: string) => void
}

export default function ProjectSelector({ projects, value, onChange }: ProjectSelectorProps) {
  return (
    <label className="project-selector" htmlFor="pm-project-select">
      <span>Project:</span>
      <select
        id="pm-project-select"
        value={value}
        onChange={(event) => {
          onChange(event.target.value)
        }}
      >
        {projects.length === 0 && <option value="">No projects</option>}
        {projects.map((project) => (
          <option key={project.id} value={project.id}>
            {project.name}
          </option>
        ))}
      </select>
    </label>
  )
}
