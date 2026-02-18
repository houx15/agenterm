const ASR_SETTINGS_KEY = 'agenterm_asr_settings'

export interface ASRSettings {
  appID: string
  accessKey: string
}

export function loadASRSettings(): ASRSettings {
  const raw = localStorage.getItem(ASR_SETTINGS_KEY)
  if (!raw) {
    return { appID: '', accessKey: '' }
  }

  try {
    const decoded = JSON.parse(raw) as Partial<ASRSettings>
    return {
      appID: typeof decoded.appID === 'string' ? decoded.appID : '',
      accessKey: typeof decoded.accessKey === 'string' ? decoded.accessKey : '',
    }
  } catch {
    return { appID: '', accessKey: '' }
  }
}

export function saveASRSettings(settings: ASRSettings): void {
  localStorage.setItem(ASR_SETTINGS_KEY, JSON.stringify(settings))
}
