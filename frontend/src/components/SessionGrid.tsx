import type { Session } from '../api/types'
import StatusDot from './StatusDot'

interface SessionWithProject {
  session: Session
  projectName: string
}

interface SessionGridProps {
  groupedSessions: Record<string, SessionWithProject[]>
  onSessionClick: (session: Session) => void
}

function agentAbbreviation(agentType: string): string {
  const normalized = (agentType || 'agent').trim()
  if (!normalized) {
    return 'AG'
  }
  return normalized.slice(0, 2).toUpperCase()
}

export default function SessionGrid({ groupedSessions, onSessionClick }: SessionGridProps) {
  const groups = Object.entries(groupedSessions)

  if (groups.length === 0) {
    return <p className="empty-text">No active sessions right now.</p>
  }

  return (
    <div className="dashboard-session-groups">
      {groups.map(([projectName, items]) => (
        <section className="dashboard-session-group" key={projectName}>
          <h4>{projectName}</h4>
          <div className="dashboard-session-grid">
            {items.map(({ session }) => (
              <button
                className="session-grid-item"
                key={session.id}
                onClick={() => onSessionClick(session)}
                title={`${session.agent_type} (${session.status})`}
                type="button"
              >
                <StatusDot status={session.status} />
                <span>{agentAbbreviation(session.agent_type)}</span>
              </button>
            ))}
          </div>
        </section>
      ))}
    </div>
  )
}
