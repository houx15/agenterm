import { useCallback, useEffect, useRef, useState } from 'react'
import { useAppContext } from '../App'
import { getSessionOutput } from '../api/client'
import type { Session } from '../api/types'
import { getWindowID } from '../api/types'
import Terminal from './Terminal'
import MarkdownPane from './MarkdownPane'
import { TerminalIcon, FileText, SplitSquareHorizontal } from './Lucide'

type ViewMode = 'tui' | 'md' | 'split'

interface AgentViewProps {
  sessionID: string
  worktreePath?: string
}

const REPLAY_MAX_CHARS = 120000

export default function AgentView({ sessionID, worktreePath }: AgentViewProps) {
  const { lastMessage, send } = useAppContext()
  const [viewMode, setViewMode] = useState<ViewMode>('tui')
  const [rawBuffer, setRawBuffer] = useState('')
  const [session, setSession] = useState<Session | null>(null)
  const sessionRef = useRef(sessionID)

  // Resolve window ID for this session
  const windowID = session ? getWindowID(session) : ''

  // When sessionID changes, reset buffer and find the session
  useEffect(() => {
    if (sessionID !== sessionRef.current) {
      sessionRef.current = sessionID
      setRawBuffer('')
    }
  }, [sessionID])

  // Bootstrap: fetch initial output snapshot
  useEffect(() => {
    if (!sessionID) return
    let cancelled = false

    const bootstrap = async () => {
      try {
        const resp = await getSessionOutput(sessionID, 1200)
        if (cancelled) return
        const snapshot = (resp.lines ?? []).map((line) => line.text ?? '').join('\r\n')
        setRawBuffer((prev) => {
          // Only use snapshot if we don't already have more data
          if (prev.length > snapshot.length) return prev
          return snapshot.slice(-REPLAY_MAX_CHARS)
        })
      } catch {
        // keep usable
      }
    }

    void bootstrap()
    return () => { cancelled = true }
  }, [sessionID])

  // Handle terminal_data messages from WebSocket
  useEffect(() => {
    if (!lastMessage || lastMessage.type !== 'terminal_data') return
    if (!windowID || lastMessage.window !== windowID) return

    setRawBuffer((prev) => {
      const next = prev + (lastMessage.text ?? '')
      return next.length > REPLAY_MAX_CHARS
        ? next.slice(next.length - REPLAY_MAX_CHARS)
        : next
    })
  }, [lastMessage, windowID])

  // Also listen for session info via windows message to get session details
  useEffect(() => {
    if (!lastMessage || lastMessage.type !== 'windows') return
    const list = Array.isArray(lastMessage.list) ? lastMessage.list : []
    const win = list.find((w) => w.session_id === sessionID)
    if (win) {
      setSession({
        id: sessionID,
        terminal_id: win.id,
        agent_type: '',
        role: win.name || '',
        status: win.status,
        human_attached: false,
        created_at: '',
        last_activity_at: '',
      })
    }
  }, [lastMessage, sessionID])

  const handleTerminalInput = useCallback(
    (keys: string) => {
      if (!windowID) return
      send({
        type: 'terminal_input',
        session_id: sessionID,
        window: windowID,
        keys,
      })
    },
    [send, sessionID, windowID],
  )

  const handleTerminalResize = useCallback(
    (cols: number, rows: number) => {
      if (!windowID) return
      send({
        type: 'terminal_resize',
        session_id: sessionID,
        window: windowID,
        cols,
        rows,
      })
    },
    [send, sessionID, windowID],
  )

  const modeBtn = (mode: ViewMode, label: string, Icon: typeof TerminalIcon) => (
    <button
      className={`flex items-center gap-1.5 px-2.5 py-1 text-xs rounded transition-colors ${
        viewMode === mode
          ? 'bg-accent/20 text-accent'
          : 'text-text-secondary hover:text-text-primary hover:bg-bg-tertiary'
      }`}
      onClick={() => setViewMode(mode)}
      type="button"
    >
      <Icon size={12} />
      {label}
    </button>
  )

  const effectiveWorktreePath = worktreePath || '/workspace'

  return (
    <div className="flex flex-col flex-1 overflow-hidden">
      {/* View mode toggle bar */}
      <div className="flex items-center gap-1 px-3 py-1.5 border-b border-border bg-bg-secondary shrink-0">
        {modeBtn('tui', 'TUI', TerminalIcon)}
        {modeBtn('md', 'MD', FileText)}
        {modeBtn('split', 'Split', SplitSquareHorizontal)}
      </div>

      {/* Content area */}
      <div className="flex-1 overflow-hidden flex">
        {viewMode === 'tui' && (
          <div className="flex-1 overflow-hidden">
            <Terminal
              sessionId={windowID || sessionID}
              history={rawBuffer}
              onInput={handleTerminalInput}
              onResize={handleTerminalResize}
            />
          </div>
        )}
        {viewMode === 'md' && (
          <div className="flex-1 overflow-hidden">
            <MarkdownPane worktreePath={effectiveWorktreePath} />
          </div>
        )}
        {viewMode === 'split' && (
          <>
            <div className="w-1/2 overflow-hidden border-r border-border">
              <Terminal
                sessionId={windowID || sessionID}
                history={rawBuffer}
                onInput={handleTerminalInput}
                onResize={handleTerminalResize}
              />
            </div>
            <div className="w-1/2 overflow-hidden">
              <MarkdownPane worktreePath={effectiveWorktreePath} />
            </div>
          </>
        )}
      </div>
    </div>
  )
}
