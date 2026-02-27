import type { Session, Task } from '../api/types'
import { getWindowID } from '../api/types'

interface TaskDAGProps {
  tasks: Task[]
  sessionsByTask: Record<string, Session>
  onOpenTask: (taskID: string) => void
}

function normalizeStatus(status: string): string {
  return (status || '').trim().toLowerCase()
}

function statusClass(status: string): string {
  const normalized = normalizeStatus(status)
  if (normalized.includes('run') || normalized.includes('working')) return 'running'
  if (normalized.includes('review')) return 'reviewing'
  if (normalized.includes('done') || normalized.includes('success') || normalized.includes('completed')) return 'done'
  if (normalized.includes('fail') || normalized.includes('error') || normalized.includes('blocked')) return 'failed'
  return 'pending'
}

export default function TaskDAG({ tasks, sessionsByTask, onOpenTask }: TaskDAGProps) {
  const taskNameByID = new Map<string, string>()
  for (const task of tasks) {
    taskNameByID.set(task.id, task.title)
  }

  return (
    <section className="pm-dag-panel">
      <div className="pm-panel-header">
        <h3>Task Graph</h3>
        <small>{tasks.length} tasks</small>
      </div>

      {tasks.length === 0 && <div className="empty-view">No tasks in this project yet.</div>}

      {tasks.length > 0 && (
        <div className="pm-dag-list" role="list">
          {tasks.map((task) => {
            const depNames = task.depends_on.map((depID) => taskNameByID.get(depID) ?? depID)
            const session = sessionsByTask[task.id]
            const clickable = Boolean(session && getWindowID(session))

            return (
              <article
                key={task.id}
                className={`pm-task-node ${statusClass(task.status)} ${clickable ? 'clickable' : ''}`.trim()}
                onClick={() => {
                  if (clickable) {
                    onOpenTask(task.id)
                  }
                }}
                role="listitem"
              >
                <div className="pm-task-title-row">
                  <strong>{task.title}</strong>
                  <span className="pm-task-status">{task.status || 'pending'}</span>
                </div>

                {depNames.length > 0 && (
                  <p className="pm-task-deps">
                    {depNames.map((name, idx) => (
                      <span key={`${task.id}-dep-${idx}`}>
                        {name}
                        {' -> '}
                      </span>
                    ))}
                    <span>{task.title}</span>
                  </p>
                )}

                <p className="pm-task-meta">{clickable ? 'Open session terminal' : 'No active session yet'}</p>
              </article>
            )
          })}
        </div>
      )}
    </section>
  )
}
