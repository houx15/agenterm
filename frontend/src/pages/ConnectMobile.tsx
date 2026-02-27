import { useMemo, useState } from 'react'
import { getToken } from '../api/client'
import { Smartphone } from '../components/Lucide'

function resolvePairURL(hostOverride: string): string {
  const token = getToken()
  const host = hostOverride.trim() || window.location.hostname
  const url = new URL(`http://${host}:8765/mobile`)
  if (token) {
    url.searchParams.set('token', token)
  }
  return url.toString()
}

export default function ConnectMobile() {
  const [host, setHost] = useState(window.location.hostname)
  const pairURL = useMemo(() => resolvePairURL(host), [host])
  const qrURL = useMemo(
    () => `https://api.qrserver.com/v1/create-qr-code/?size=280x280&data=${encodeURIComponent(pairURL)}`,
    [pairURL],
  )

  return (
    <section className="page-block connect-mobile-page">
      <header className="dashboard-hero">
        <div>
          <h2>Connect Mobile</h2>
          <p className="empty-text">Scan the QR code with your phone to open the companion view.</p>
        </div>
      </header>

      <section className="dashboard-section connect-mobile-card">
        <label className="settings-field">
          <span>Desktop Host/IP</span>
          <input
            onChange={(event) => setHost(event.target.value)}
            placeholder="192.168.1.23"
            type="text"
            value={host}
          />
        </label>

        <div className="connect-mobile-qr-wrap">
          <img alt="Pair mobile device QR code" className="connect-mobile-qr" src={qrURL} />
        </div>

        <label className="settings-field">
          <span>Pair Link</span>
          <textarea readOnly value={pairURL} />
        </label>

        <div className="settings-actions">
          <button className="secondary-btn" onClick={() => void navigator.clipboard.writeText(pairURL)} type="button">
            <Smartphone size={14} />
            <span>Copy Link</span>
          </button>
        </div>
      </section>
    </section>
  )
}
