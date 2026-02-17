import { useAppContext } from '../App'

export default function Dashboard() {
  const app = useAppContext()
  return (
    <section className="page-block">
      <h2>Dashboard</h2>
      <p>Overview placeholder for projects and activity.</p>
      <div className="stats-grid">
        <article>
          <h3>Connection</h3>
          <p>{app.connected ? 'Connected' : 'Disconnected'}</p>
        </article>
        <article>
          <h3>Sessions</h3>
          <p>{app.windows.length}</p>
        </article>
      </div>
    </section>
  )
}
