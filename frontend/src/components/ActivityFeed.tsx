export interface DashboardActivity {
  id: string
  timestamp: string
  text: string
}

interface ActivityFeedProps {
  items: DashboardActivity[]
}

function formatRelativeTime(isoTimestamp: string): string {
  const time = new Date(isoTimestamp).getTime()
  if (Number.isNaN(time)) {
    return 'just now'
  }

  const diffMs = Date.now() - time
  const seconds = Math.max(1, Math.floor(diffMs / 1000))

  if (seconds < 60) {
    return `${seconds}s ago`
  }

  const minutes = Math.floor(seconds / 60)
  if (minutes < 60) {
    return `${minutes}m ago`
  }

  const hours = Math.floor(minutes / 60)
  if (hours < 24) {
    return `${hours}h ago`
  }

  const days = Math.floor(hours / 24)
  return `${days}d ago`
}

export default function ActivityFeed({ items }: ActivityFeedProps) {
  if (items.length === 0) {
    return <p className="empty-text">No recent activity yet.</p>
  }

  return (
    <ul className="activity-feed-list">
      {items.map((item) => (
        <li key={item.id}>
          <span>{item.text}</span>
          <small>{formatRelativeTime(item.timestamp)}</small>
        </li>
      ))}
    </ul>
  )
}
