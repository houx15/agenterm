import { useCallback, useEffect, useState } from 'react'
import { useAppContext } from '../App'
import {
  listSessions,
  listRequirements,
  createRequirement,
} from '../api/client'
import type { Requirement } from '../api/client'
import type { Session } from '../api/types'
import AgentSidebar from './AgentSidebar'
import StatusGraph from './StatusGraph'
import StageControls from './StageControls'
import AgentView from './AgentView'

// ---------------------------------------------------------------------------
// EmptyWorkspace — shown when no active requirement exists
// ---------------------------------------------------------------------------

function EmptyWorkspace({ onSubmit }: { onSubmit: (title: string) => void }) {
  const [title, setTitle] = useState('')

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    if (!title.trim()) return
    onSubmit(title.trim())
    setTitle('')
  }

  return (
    <div className="flex flex-col items-center justify-center h-full gap-6 p-8">
      <div className="text-center">
        <h2 className="text-xl font-semibold text-text-primary mb-2">No active requirements</h2>
        <p className="text-sm text-text-secondary">Start by describing what you want to build.</p>
      </div>
      <form onSubmit={handleSubmit} className="flex gap-2 w-full max-w-lg">
        <input
          type="text"
          value={title}
          onChange={(e) => setTitle(e.target.value)}
          placeholder="What do you want to build?"
          className="flex-1 rounded border border-border bg-bg-tertiary px-4 py-3 text-sm text-text-primary placeholder:text-text-secondary/50 focus:border-accent focus:outline-none"
          autoFocus
        />
        <button
          type="submit"
          disabled={!title.trim()}
          className="rounded bg-accent px-6 py-3 text-sm font-medium text-white hover:bg-accent/80 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
        >
          Start
        </button>
      </form>
    </div>
  )
}

// ---------------------------------------------------------------------------
// WorkspaceView
// ---------------------------------------------------------------------------

export default function WorkspaceView() {
  const { selectedProjectID } = useAppContext()
  const [sessions, setSessions] = useState<Session[]>([])
  const [selectedSessionID, setSelectedSessionID] = useState<string | null>(null)
  const [requirement, setRequirement] = useState<Requirement | null>(null)

  const fetchData = useCallback(async () => {
    if (!selectedProjectID) {
      setSessions([])
      setRequirement(null)
      return
    }
    try {
      const [sessionList, reqList] = await Promise.all([
        listSessions<Session[]>({ projectID: selectedProjectID }),
        listRequirements(selectedProjectID),
      ])
      setSessions(sessionList)

      // Find the first non-done requirement as the "active" one
      const activeReq = reqList.find((r) => r.status !== 'done') ?? null
      setRequirement(activeReq)

      // Auto-select first session if current selection is invalid
      if (sessionList.length > 0) {
        const hasSelected = selectedSessionID && sessionList.some((s) => s.id === selectedSessionID)
        if (!hasSelected) {
          setSelectedSessionID(sessionList[0].id)
        }
      }
    } catch {
      // keep workspace usable
    }
  }, [selectedProjectID, selectedSessionID])

  useEffect(() => {
    void fetchData()
    const timer = window.setInterval(() => void fetchData(), 5000)
    return () => window.clearInterval(timer)
  }, [fetchData])

  const handleFirstDemand = useCallback(
    async (title: string) => {
      if (!selectedProjectID) return
      try {
        await createRequirement(selectedProjectID, { title })
        await fetchData()
      } catch {
        // keep usable
      }
    },
    [selectedProjectID, fetchData],
  )

  const handleNodeClick = useCallback((_type: string, _id?: string) => {
    // Future: navigate to specific stage or worktree view
  }, [])

  if (!selectedProjectID) {
    return (
      <div className="flex items-center justify-center h-full text-text-secondary">
        Select a project to get started
      </div>
    )
  }

  // Empty state: no requirements yet
  if (!requirement) {
    return <EmptyWorkspace onSubmit={(title) => void handleFirstDemand(title)} />
  }

  // Find worktree path from selected session
  const selectedSession = sessions.find((s) => s.id === selectedSessionID)
  const worktreePath = selectedSession
    ? (selectedSession as unknown as Record<string, string>).worktree_path
    : undefined

  return (
    <div className="flex flex-1 overflow-hidden h-full">
      <AgentSidebar
        sessions={sessions}
        selectedSessionID={selectedSessionID}
        onSelectSession={setSelectedSessionID}
      />
      <div className="flex-1 flex flex-col overflow-hidden">
        <StatusGraph
          currentStage={requirement.status}
          onNodeClick={handleNodeClick}
        />
        <StageControls
          requirementID={requirement.id}
          currentStage={requirement.status}
          onTransition={() => void fetchData()}
        />
        {selectedSessionID ? (
          <AgentView
            sessionID={selectedSessionID}
            worktreePath={worktreePath}
          />
        ) : (
          <div className="flex items-center justify-center flex-1 text-text-secondary text-sm">
            Select a session from the sidebar
          </div>
        )}
      </div>
    </div>
  )
}
