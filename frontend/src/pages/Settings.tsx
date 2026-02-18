import { useState } from 'react'
import { loadASRSettings, saveASRSettings } from '../settings/asr'

export default function Settings() {
  const [settings, setSettings] = useState(() => loadASRSettings())
  const [saved, setSaved] = useState(false)

  const onSave = () => {
    saveASRSettings({
      appID: settings.appID.trim(),
      accessKey: settings.accessKey.trim(),
    })
    setSaved(true)
    window.setTimeout(() => setSaved(false), 1500)
  }

  return (
    <section className="page-block settings-page">
      <h2>Settings</h2>
      <div className="settings-card">
        <h3>Volcengine ASR</h3>
        <p>Configure speech-to-text credentials for PM Chat microphone input.</p>

        <label className="settings-field" htmlFor="asr-app-id">
          <span>App ID</span>
          <input
            id="asr-app-id"
            value={settings.appID}
            onChange={(event) => setSettings((prev) => ({ ...prev, appID: event.target.value }))}
            placeholder="volc app id"
          />
        </label>

        <label className="settings-field" htmlFor="asr-access-key">
          <span>Access Key</span>
          <input
            id="asr-access-key"
            type="password"
            value={settings.accessKey}
            onChange={(event) => setSettings((prev) => ({ ...prev, accessKey: event.target.value }))}
            placeholder="volc access key"
          />
        </label>

        <div className="settings-actions">
          <button className="primary-btn" type="button" onClick={onSave}>
            Save
          </button>
          {saved && <small>Saved</small>}
        </div>
      </div>
    </section>
  )
}
