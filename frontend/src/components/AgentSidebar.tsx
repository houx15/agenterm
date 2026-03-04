import { useMemo, useState } from 'react'
import type { Session } from '../api/types'
import { ChevronDown, ChevronRight, Bot, TerminalIcon } from './Lucide'

interface AgentSidebarProps {
  sessions: Session[]
  selectedSessionID: string | null
  onSelectSession: (id: string) => void
}

function needsResponse(status: string): boolean {
  const s = (status || '').toLowerCase()
  return ['blocked', 'waiting', 'needs_input', 'human_takeover', 'waiting_review', 'suspended'].includes(s)
}

function statusDotClass(status: string): string {
  const s = (status || '').toLowerCase()
  if (['working', 'running', 'executing', 'busy', 'active'].includes(s)) return 'bg-status-working'
  if (needsResponse(s)) return 'bg-status-waiting'
  if (['error', 'failed', 'terminated'].includes(s)) return 'bg-status-error'
  return 'bg-status-idle'
}

function isPlanner(session: Session): boolean {
  const role = (session.role || '').toLowerCase()
  return role.includes('planner') || role.includes('orchestrator') || role.includes('pm')
}

interface WorktreeGroup {
  worktreeID: string
  label: string
  sessions: Session[]
}

export default function AgentSidebar({ sessions, selectedSessionID, onSelectSession }: AgentSidebarProps) {
  const [collapsedGroups, setCollapsedGroups] = useState<Record<string, boolean>>({})

  const plannerSessions = useMemo(() => sessions.filter(isPlanner), [sessions])

  const worktreeGroups = useMemo(() => {
    const nonPlanners = sessions.filter((s) => !isPlanner(s))
    const groupMap = new Map<string, Session[]>()

    for (const session of nonPlanners) {
      const key = (session as unknown as Record<string, string>).worktree_id || 'default'
      const group = groupMap.get(key)
      if (group) {
        group.push(session)
      } else {
        groupMap.set(key, [session])
      }
    }

    const groups: WorktreeGroup[] = []
    for (const [worktreeID, groupSessions] of groupMap.entries()) {
      groups.push({
        worktreeID,
        label: worktreeID === 'default' ? 'Main' : worktreeID,
        sessions: groupSessions,
      })
    }
    return groups
  }, [sessions])

  const toggleGroup = (id: string) => {
    setCollapsedGroups((prev) => ({ ...prev, [id]: !prev[id] }))
  }

  const renderSession = (session: Session) => {
    const isSelected = session.id === selectedSessionID
    const agentName = session.role || session.agent_type || 'Agent'
    const alert = needsResponse(session.status)

    return (
      <button
        key={session.id}
        className={`flex items-center gap-2 w-full px-3 py-1.5 text-left text-xs transition-colors rounded ${
          isSelected
            ? 'bg-accent/20 text-accent'
            : 'text-text-primary hover:bg-bg-tertiary'
        }`}
        onClick={() => onSelectSession(session.id)}
        title={`${agentName} - ${session.status}`}
        type="button"
      >
        <span className={`inline-block w-2 h-2 rounded-full shrink-0 ${statusDotClass(session.status)}`} />
        {isPlanner(session) ? <Bot size={12} /> : <TerminalIcon size={12} />}
        <span className="flex-1 truncate">{agentName}</span>
        {alert && <span className="text-status-error text-[10px] shrink-0">!</span>}
        <span className="text-[10px] text-text-secondary shrink-0">{session.role ? session.role.split('_')[0] : ''}</span>
      </button>
    )
  }

  return (
    <aside className="w-48 border-r border-border bg-bg-secondary overflow-y-auto shrink-0">
      {/* Planner section */}
      {plannerSessions.length > 0 && (
        <div className="border-b border-border">
          <div className="px-3 pt-3 pb-1 text-[10px] font-semibold tracking-widest text-text-secondary uppercase">
            Planner
          </div>
          <div className="flex flex-col gap-0.5 px-1 py-1">
            {plannerSessions.map(renderSession)}
          </div>
        </div>
      )}

      {/* Worktree groups */}
      {worktreeGroups.map((group) => {
        const isCollapsed = !!collapsedGroups[group.worktreeID]
        return (
          <div key={group.worktreeID} className="border-b border-border/50">
            <button
              className="flex items-center gap-1 w-full px-3 py-2 text-[10px] font-semibold tracking-widest text-text-secondary uppercase hover:bg-bg-tertiary transition-colors"
              onClick={() => toggleGroup(group.worktreeID)}
              type="button"
            >
              {isCollapsed ? <ChevronRight size={10} /> : <ChevronDown size={10} />}
              <span className="truncate">{group.label}</span>
              <span className="ml-auto text-[10px] tabular-nums">{group.sessions.length}</span>
            </button>
            {!isCollapsed && (
              <div className="flex flex-col gap-0.5 px-1 pb-1">
                {group.sessions.map(renderSession)}
              </div>
            )}
          </div>
        )
      })}

      {sessions.length === 0 && (
        <div className="flex items-center justify-center h-full text-text-secondary text-xs p-4">
          No active sessions
        </div>
      )}
    </aside>
  )
}
