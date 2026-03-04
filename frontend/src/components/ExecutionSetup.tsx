import { useEffect, useState } from 'react'
import { getAgentStatuses, launchExecution } from '../api/client'
import type { AgentStatus } from '../api/client'
import { GitBranch, Play } from './Lucide'

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

interface BlueprintTask {
  id: string
  title: string
  description: string
  completion_criteria: string[]
  worktree_branch: string
  agent_type: string
  depends_on?: string[]
}

interface Blueprint {
  tasks: BlueprintTask[]
}

interface ExecutionSetupProps {
  requirementID: string
  blueprint: Blueprint
  onLaunch: () => void
}

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

export default function ExecutionSetup({ requirementID, blueprint, onLaunch }: ExecutionSetupProps) {
  const [agentStatuses, setAgentStatuses] = useState<AgentStatus[]>([])
  const [assignments, setAssignments] = useState<Record<string, string>>({})
  const [launching, setLaunching] = useState(false)
  const [error, setError] = useState('')

  // Load agent statuses for capacity info
  useEffect(() => {
    let cancelled = false
    getAgentStatuses()
      .then((statuses) => {
        if (cancelled) return
        setAgentStatuses(statuses)
        // Initialize assignments with blueprint defaults
        const initial: Record<string, string> = {}
        for (const task of blueprint.tasks) {
          // Try to match the task's agent_type to an available agent
          const match = statuses.find((s) => s.id === task.agent_type)
          initial[task.id] = match ? match.id : (statuses[0]?.id ?? '')
        }
        setAssignments(initial)
      })
      .catch(() => {
        // Still allow viewing the blueprint
      })
    return () => { cancelled = true }
  }, [blueprint.tasks])

  const getCapacityLabel = (agentId: string): string => {
    const status = agentStatuses.find((s) => s.agent_id === agentId)
    if (!status) return ''
    const available = status.capacity - status.busy
    return `(${available}/${status.capacity} available)`
  }

  const handleAssignment = (taskId: string, agentId: string) => {
    setAssignments((prev) => ({ ...prev, [taskId]: agentId }))
  }

  const handleLaunch = async () => {
    setLaunching(true)
    setError('')
    try {
      await launchExecution(requirementID)
      onLaunch()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to launch execution')
    } finally {
      setLaunching(false)
    }
  }

  // Build dependency label map
  const taskTitleMap: Record<string, string> = {}
  for (const task of blueprint.tasks) {
    taskTitleMap[task.id] = task.title
  }

  return (
    <div className="h-full overflow-y-auto p-6">
      <div className="max-w-3xl mx-auto">
        {/* Header */}
        <div className="mb-6">
          <h2 className="text-lg font-semibold text-text-primary mb-1">Execution Setup</h2>
          <p className="text-sm text-text-secondary">
            Review the blueprint and assign agents before launching.
          </p>
        </div>

        {/* Task List */}
        <div className="space-y-4 mb-8">
          {blueprint.tasks.map((task, index) => (
            <div
              key={task.id}
              className="rounded-lg border border-border bg-bg-secondary p-5"
            >
              <div className="flex items-start gap-3 mb-3">
                <span className="flex-shrink-0 w-7 h-7 rounded-full bg-accent/20 text-accent text-sm font-medium flex items-center justify-center">
                  {index + 1}
                </span>
                <div className="flex-1 min-w-0">
                  <h3 className="text-sm font-medium text-text-primary mb-1">{task.title}</h3>
                  <p className="text-xs text-text-secondary leading-relaxed">{task.description}</p>
                </div>
              </div>

              {/* Completion criteria */}
              {task.completion_criteria.length > 0 && (
                <div className="ml-10 mb-3">
                  <span className="text-xs text-text-secondary font-medium block mb-1">Completion criteria:</span>
                  <ul className="text-xs text-text-secondary space-y-0.5 list-disc ml-4">
                    {task.completion_criteria.map((c, i) => (
                      <li key={i}>{c}</li>
                    ))}
                  </ul>
                </div>
              )}

              {/* Dependencies */}
              {task.depends_on && task.depends_on.length > 0 && (
                <div className="ml-10 mb-3">
                  <span className="text-xs text-text-secondary font-medium">Depends on: </span>
                  <span className="text-xs text-accent">
                    {task.depends_on.map((id) => taskTitleMap[id] ?? id).join(', ')}
                  </span>
                </div>
              )}

              {/* Branch & Agent assignment */}
              <div className="ml-10 flex flex-wrap items-center gap-4">
                <div className="flex items-center gap-1.5 text-xs text-text-secondary">
                  <GitBranch size={14} className="text-accent" />
                  <span className="font-mono">{task.worktree_branch}</span>
                </div>

                <div className="flex items-center gap-2">
                  <label className="text-xs text-text-secondary">Agent:</label>
                  <select
                    value={assignments[task.id] ?? ''}
                    onChange={(e) => handleAssignment(task.id, e.target.value)}
                    className="rounded border border-border bg-bg-tertiary px-2 py-1 text-xs text-text-primary focus:border-accent focus:outline-none"
                  >
                    {agentStatuses.length === 0 && (
                      <option value="">No agents available</option>
                    )}
                    {agentStatuses.map((agent) => (
                      <option key={agent.agent_id} value={agent.agent_id}>
                        {agent.agent_name} {getCapacityLabel(agent.agent_id)}
                      </option>
                    ))}
                  </select>
                </div>
              </div>
            </div>
          ))}
        </div>

        {/* Error */}
        {error && (
          <div className="rounded border border-status-error/50 bg-status-error/10 text-status-error text-sm p-3 mb-4">
            {error}
          </div>
        )}

        {/* Launch button */}
        <div className="flex justify-end">
          <button
            onClick={() => void handleLaunch()}
            disabled={launching || blueprint.tasks.length === 0}
            className="flex items-center gap-2 rounded bg-accent px-8 py-3 text-sm font-medium text-white hover:bg-accent/80 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
          >
            {launching ? (
              <>Launching...</>
            ) : (
              <>
                <Play size={16} />
                Launch All ({blueprint.tasks.length} task{blueprint.tasks.length !== 1 ? 's' : ''})
              </>
            )}
          </button>
        </div>
      </div>
    </div>
  )
}
