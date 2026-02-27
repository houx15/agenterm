/**
 * Maps a session status string to a CSS class name for status-dot styling.
 *
 * - 'working' | 'running' | 'executing' | 'busy' | 'active' → 'working'
 * - 'waiting' | 'waiting_review' | 'human_takeover' | 'blocked' | 'needs_input' → 'waiting'
 * - 'error' | 'failed' → 'error'
 * - anything else → 'idle'
 */
export function getStatusClass(status: string): string {
  switch (status) {
    case 'working':
    case 'running':
    case 'executing':
    case 'busy':
    case 'active':
      return 'working'
    case 'waiting':
    case 'waiting_review':
    case 'human_takeover':
    case 'blocked':
    case 'needs_input':
      return 'waiting'
    case 'error':
    case 'failed':
      return 'error'
    default:
      return 'idle'
  }
}
