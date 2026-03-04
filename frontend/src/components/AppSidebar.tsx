import { useCallback, useEffect, useState } from 'react'
import { useAppContext } from '../App'
import { listProjects, getAgentStatuses } from '../api/client'
import type { AgentStatus } from '../api/client'
import type { Project } from '../api/types'
import { ChevronLeft, ChevronRight, Plus, Settings } from './Lucide'

interface AppSidebarProps {
  onNewProject: () => void
  onOpenSettings: () => void
}

function projectInitials(name: string): string {
  return name
    .split(/[\s_-]+/)
    .slice(0, 2)
    .map((w) => w[0]?.toUpperCase() ?? '')
    .join('')
}

export default function AppSidebar({ onNewProject, onOpenSettings }: AppSidebarProps) {
  const { selectedProjectID, setSelectedProjectID } = useAppContext()
  const [projects, setProjects] = useState<Project[]>([])
  const [agents, setAgents] = useState<AgentStatus[]>([])
  const [folded, setFolded] = useState(false)

  const fetchData = useCallback(async () => {
    try {
      const [projectList, agentList] = await Promise.all([
        listProjects<Project[]>({ status: 'active' }),
        getAgentStatuses(),
      ])
      setProjects(projectList)
      setAgents(agentList)
    } catch {
      // keep sidebar usable on fetch failure
    }
  }, [])

  // Fetch on mount + poll every 5s
  useEffect(() => {
    void fetchData()
    const timer = window.setInterval(() => void fetchData(), 5000)
    return () => window.clearInterval(timer)
  }, [fetchData])

  // Auto-select first project if none selected
  useEffect(() => {
    if (!selectedProjectID && projects.length > 0) {
      setSelectedProjectID(projects[0].id)
    }
  }, [selectedProjectID, projects, setSelectedProjectID])

  return (
    <aside
      className={`flex flex-col border-r border-border bg-bg-secondary transition-all duration-200 shrink-0 ${
        folded ? 'w-12' : 'w-60'
      }`}
    >
      {/* Header */}
      <div className="flex items-center justify-between px-3 py-3 border-b border-border">
        {!folded && (
          <span className="text-sm font-bold tracking-wider text-text-secondary uppercase">
            agenterm
          </span>
        )}
        <button
          className="p-1 rounded text-text-secondary hover:text-text-primary hover:bg-bg-tertiary transition-colors"
          onClick={() => setFolded((prev) => !prev)}
          title={folded ? 'Expand sidebar' : 'Collapse sidebar'}
          type="button"
        >
          {folded ? <ChevronRight size={14} /> : <ChevronLeft size={14} />}
        </button>
      </div>

      {/* Projects section */}
      <div className="flex-1 overflow-y-auto">
        {!folded && (
          <div className="px-3 pt-3 pb-1 text-[10px] font-semibold tracking-widest text-text-secondary uppercase">
            Projects
          </div>
        )}
        <div className="flex flex-col gap-0.5 px-1 py-1">
          {projects.map((project) => {
            const isSelected = project.id === selectedProjectID
            return (
              <button
                key={project.id}
                className={`flex items-center gap-2 w-full rounded px-2 py-1.5 text-left text-sm transition-colors ${
                  isSelected
                    ? 'bg-accent/20 text-accent'
                    : 'text-text-primary hover:bg-bg-tertiary'
                }`}
                onClick={() => setSelectedProjectID(project.id)}
                title={project.name}
                type="button"
              >
                {folded ? (
                  <span className="mx-auto text-xs font-bold">
                    {projectInitials(project.name)}
                  </span>
                ) : (
                  <>
                    <span className="flex-1 truncate">{project.name}</span>
                  </>
                )}
              </button>
            )
          })}
        </div>

        {/* New Project button */}
        <button
          className="flex items-center gap-2 w-full px-3 py-2 text-sm text-text-secondary hover:text-accent hover:bg-bg-tertiary transition-colors"
          onClick={onNewProject}
          type="button"
        >
          <Plus size={14} />
          {!folded && <span>New Project</span>}
        </button>
      </div>

      {/* Agents section */}
      <div className="border-t border-border">
        {!folded && (
          <div className="px-3 pt-3 pb-1 text-[10px] font-semibold tracking-widest text-text-secondary uppercase">
            Agents
          </div>
        )}
        <div className="flex flex-col gap-0.5 px-1 py-1">
          {agents.map((agent) => {
            const utilization = agent.max_parallel > 0 ? agent.busy / agent.max_parallel : 0
            const dotColor =
              utilization >= 1
                ? 'bg-status-error'
                : utilization > 0
                  ? 'bg-status-working'
                  : 'bg-status-idle'

            return (
              <div
                key={agent.id}
                className="flex items-center gap-2 px-2 py-1.5 rounded text-sm text-text-secondary"
                title={`${agent.name}: ${agent.busy}/${agent.max_parallel} busy`}
              >
                <span className={`inline-block w-2 h-2 rounded-full shrink-0 ${dotColor}`} />
                {!folded && (
                  <>
                    <span className="flex-1 truncate">{agent.name}</span>
                    <span className="text-xs tabular-nums">
                      {agent.busy}/{agent.max_parallel}
                    </span>
                  </>
                )}
              </div>
            )
          })}
          {agents.length === 0 && !folded && (
            <div className="px-2 py-1.5 text-xs text-text-secondary">No agents configured</div>
          )}
        </div>
      </div>

      {/* Footer: Settings */}
      <div className="border-t border-border px-1 py-2">
        <button
          className="flex items-center gap-2 w-full px-2 py-1.5 rounded text-sm text-text-secondary hover:text-text-primary hover:bg-bg-tertiary transition-colors"
          onClick={onOpenSettings}
          type="button"
        >
          <Settings size={14} />
          {!folded && <span>Settings</span>}
        </button>
      </div>
    </aside>
  )
}
