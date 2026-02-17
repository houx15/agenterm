import type { SessionStatus } from '../api/types'

interface StatusDotProps {
  status: SessionStatus
}

export default function StatusDot({ status }: StatusDotProps) {
  return <span className={`status-dot ${status || 'idle'}`} aria-label={`status ${status || 'idle'}`} />
}
