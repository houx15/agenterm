import Terminal from './Terminal'
import { X } from './Lucide'
import type { Session } from '../api/types'
import { getStatusClass } from '../utils/getStatusClass'

interface TerminalGridProps {
  sessions: Session[]
  rawBuffers: Record<string, string>
  activeWindowID: string | null
  onTerminalInput: (windowID: string, sessionID: string, keys: string) => void
  onTerminalResize: (windowID: string, sessionID: string, cols: number, rows: number) => void
  onClosePane: (windowID: string) => void
  onFocusPane: (windowID: string) => void
}

export default function TerminalGrid({
  sessions,
  rawBuffers,
  activeWindowID,
  onTerminalInput,
  onTerminalResize,
  onClosePane,
  onFocusPane,
}: TerminalGridProps) {
  const terminalSessions = sessions.filter(
    (s) => s.tmux_window_id && s.agent_type !== 'orchestrator',
  )

  if (terminalSessions.length === 0) {
    return (
      <div className="terminal-grid">
        <div className="empty-state">No active agent terminals</div>
      </div>
    )
  }

  return (
    <div className="terminal-grid">
      {terminalSessions.map((session) => {
        const windowID = session.tmux_window_id!
        const statusClass = getStatusClass(session.status)

        return (
          <div
            key={windowID}
            className={`terminal-pane ${windowID === activeWindowID ? 'active' : ''}`}
            onClick={() => onFocusPane(windowID)}
          >
            <div className="terminal-pane-header">
              <span className={`sidebar-status-dot ${statusClass}`} />
              <span className="pane-name">{session.role || session.agent_type}</span>
              <small style={{ color: 'var(--text-tertiary)', fontSize: '11px' }}>
                {session.status}
              </small>
              <button
                className="pane-close"
                onClick={(e) => {
                  e.stopPropagation()
                  onClosePane(windowID)
                }}
              >
                <X size={12} />
              </button>
            </div>
            <Terminal
              sessionId={windowID}
              history={rawBuffers[windowID] || ''}
              onInput={(keys) => onTerminalInput(windowID, session.id, keys)}
              onResize={(cols, rows) => onTerminalResize(windowID, session.id, cols, rows)}
            />
          </div>
        )
      })}
    </div>
  )
}
