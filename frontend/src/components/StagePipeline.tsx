import type { Session } from '../api/types'

interface StagePipelineProps {
  currentPhase: string
  sessions?: Session[]
  collapsed?: boolean
  onToggle?: () => void
}

const STAGES = ['brainstorm', 'plan', 'build', 'test', 'summarize'] as const

interface WorktreeGroup {
  taskId: string
  entries: { role: string; agentType: string; status: string }[]
}

function buildWorktreeGroups(sessions: Session[]): WorktreeGroup[] {
  const relevant = sessions.filter(
    (s) => s.role.toLowerCase().includes('coder') || s.role.toLowerCase().includes('reviewer'),
  )

  const grouped = new Map<string, { role: string; agentType: string; status: string }[]>()
  for (const s of relevant) {
    const taskId = s.task_id ?? 'unknown'
    if (!grouped.has(taskId)) {
      grouped.set(taskId, [])
    }
    grouped.get(taskId)!.push({
      role: s.role,
      agentType: s.agent_type,
      status: s.status,
    })
  }

  const result: WorktreeGroup[] = []
  for (const [taskId, entries] of grouped) {
    result.push({ taskId, entries })
  }
  return result
}

export default function StagePipeline({ currentPhase, sessions, collapsed, onToggle }: StagePipelineProps) {
  const activeIndex = STAGES.findIndex((s) => s === currentPhase.trim().toLowerCase())

  const buildStageActive = activeIndex === STAGES.indexOf('build')
  const buildDetails = buildStageActive && sessions ? buildWorktreeGroups(sessions) : []

  return (
    <div className="stage-pipeline-wrapper">
      <div className="stage-pipeline-header" onClick={onToggle}>
        <span>{collapsed ? '\u25b8 Roadmap' : '\u25be Roadmap'}</span>
      </div>

      {!collapsed && (
        <>
          <div className="stage-pipeline">
            {STAGES.map((stage, idx) => {
              let variant: string
              if (idx < activeIndex) variant = 'done'
              else if (idx === activeIndex) variant = 'active'
              else variant = 'pending'

              return (
                <span key={stage}>
                  {idx > 0 && <span className="stage-arrow">{'\u2192'}</span>}
                  <span className={`stage-chip ${variant}`}>{stage}</span>
                </span>
              )
            })}
          </div>

          {buildStageActive && buildDetails.length > 0 && (
            <div className="stage-build-detail">
              {buildDetails.map((group) =>
                group.entries.map((entry, entryIdx) => (
                  <div key={`${group.taskId}-${entryIdx}`}>
                    task-{group.taskId}: {entry.role} ({entry.agentType}) ● {entry.status}
                  </div>
                )),
              )}
            </div>
          )}
        </>
      )}
    </div>
  )
}
