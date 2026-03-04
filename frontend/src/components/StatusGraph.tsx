import { GitBranch } from './Lucide'

interface WorktreeInfo {
  id: string
  name: string
  status: string
}

interface StatusGraphProps {
  currentStage: string
  worktrees?: WorktreeInfo[]
  onNodeClick?: (type: string, id?: string) => void
}

const STAGES = ['plan', 'build', 'review', 'merge', 'test'] as const

function stageLabel(stage: string): string {
  return stage.charAt(0).toUpperCase() + stage.slice(1)
}

function stageState(stage: string, currentStage: string): 'completed' | 'current' | 'future' {
  const currentIndex = STAGES.indexOf(currentStage as (typeof STAGES)[number])
  const stageIndex = STAGES.indexOf(stage as (typeof STAGES)[number])
  if (currentIndex < 0) return 'future'
  if (stageIndex < currentIndex) return 'completed'
  if (stageIndex === currentIndex) return 'current'
  return 'future'
}

function worktreeStatusDot(status: string): string {
  const s = status.toLowerCase()
  if (['working', 'running', 'active'].includes(s)) return 'bg-status-working'
  if (['waiting', 'blocked', 'needs_input'].includes(s)) return 'bg-status-waiting'
  if (['error', 'failed'].includes(s)) return 'bg-status-error'
  if (['done', 'completed', 'merged'].includes(s)) return 'bg-status-working'
  return 'bg-status-idle'
}

export default function StatusGraph({ currentStage, worktrees, onNodeClick }: StatusGraphProps) {
  return (
    <div className="flex flex-col gap-2 px-4 py-3 border-b border-border bg-bg-secondary shrink-0">
      {/* Pipeline row */}
      <div className="flex items-center gap-0">
        {STAGES.map((stage, index) => {
          const state = stageState(stage, currentStage)
          return (
            <div key={stage} className="flex items-center">
              {/* Stage node */}
              <button
                className={`relative flex items-center justify-center rounded-md px-3 py-1.5 text-xs font-medium transition-all ${
                  state === 'completed'
                    ? 'bg-status-working/20 text-status-working border border-status-working/30'
                    : state === 'current'
                      ? 'bg-accent/20 text-accent border border-accent/50 shadow-[0_0_8px_rgba(124,92,255,0.3)]'
                      : 'bg-bg-tertiary text-text-secondary border border-border'
                } ${onNodeClick ? 'cursor-pointer hover:brightness-110' : ''}`}
                onClick={() => onNodeClick?.(stage)}
                type="button"
              >
                {stageLabel(stage)}
                {state === 'current' && (
                  <span className="absolute -top-0.5 -right-0.5 w-2 h-2 rounded-full bg-accent animate-pulse" />
                )}
              </button>

              {/* Connector line */}
              {index < STAGES.length - 1 && (
                <div
                  className={`w-6 h-px mx-0.5 ${
                    stageState(STAGES[index + 1], currentStage) !== 'future'
                      ? 'bg-status-working/50'
                      : 'bg-border'
                  }`}
                />
              )}
            </div>
          )
        })}
      </div>

      {/* Worktree sub-nodes under Build stage */}
      {worktrees && worktrees.length > 0 && (
        <div className="flex items-start gap-2 pl-[calc(theme(spacing.3)+theme(spacing.6)+theme(spacing.0.5))]">
          <div className="w-px h-4 bg-border ml-3" />
          <div className="flex flex-wrap gap-1.5 pt-1">
            {worktrees.map((wt) => (
              <button
                key={wt.id}
                className="flex items-center gap-1.5 rounded border border-border bg-bg-tertiary px-2 py-1 text-xs text-text-secondary hover:bg-bg-secondary transition-colors"
                onClick={() => onNodeClick?.('worktree', wt.id)}
                title={`${wt.name} (${wt.status})`}
                type="button"
              >
                <GitBranch size={11} />
                <span className={`inline-block w-1.5 h-1.5 rounded-full ${worktreeStatusDot(wt.status)}`} />
                <span className="truncate max-w-[120px]">{wt.name}</span>
              </button>
            ))}
          </div>
        </div>
      )}
    </div>
  )
}
