const APPEARANCE_KEY = 'agenterm:appearance'

export interface AppearanceSettings {
  theme: string
  fontLatin: string
  fontCJK: string
  fontTerminal: string
  terminalFontSize: number
  language: string
}

const defaults: AppearanceSettings = {
  theme: 'dark',
  fontLatin: '',
  fontCJK: '',
  fontTerminal: '',
  terminalFontSize: 13,
  language: 'en',
}

export function loadAppearanceSettings(): AppearanceSettings {
  const raw = localStorage.getItem(APPEARANCE_KEY)
  if (!raw) return { ...defaults }

  try {
    const parsed = JSON.parse(raw) as Partial<AppearanceSettings>
    return {
      theme: typeof parsed.theme === 'string' ? parsed.theme : defaults.theme,
      fontLatin: typeof parsed.fontLatin === 'string' ? parsed.fontLatin : defaults.fontLatin,
      fontCJK: typeof parsed.fontCJK === 'string' ? parsed.fontCJK : defaults.fontCJK,
      fontTerminal: typeof parsed.fontTerminal === 'string' ? parsed.fontTerminal : defaults.fontTerminal,
      terminalFontSize:
        typeof parsed.terminalFontSize === 'number' && parsed.terminalFontSize >= 12 && parsed.terminalFontSize <= 20
          ? parsed.terminalFontSize
          : defaults.terminalFontSize,
      language: typeof parsed.language === 'string' ? parsed.language : defaults.language,
    }
  } catch {
    return { ...defaults }
  }
}

export function saveAppearanceSettings(settings: AppearanceSettings): void {
  localStorage.setItem(APPEARANCE_KEY, JSON.stringify(settings))
}

export function resolveEffectiveTheme(theme: string): 'dark' | 'light' | 'monokai' | 'one-dark-pro' {
  if (theme === 'system') {
    return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
  }
  if (['dark', 'light', 'monokai', 'one-dark-pro'].includes(theme)) {
    return theme as 'dark' | 'light' | 'monokai' | 'one-dark-pro'
  }
  return 'dark'
}

export function applyAppearanceToDOM(settings: AppearanceSettings): void {
  const root = document.documentElement
  root.setAttribute('data-theme', resolveEffectiveTheme(settings.theme))

  if (settings.fontLatin) {
    root.style.setProperty('--font-sans', `'${settings.fontLatin}', var(--font-sans-fallback)`)
  } else {
    root.style.removeProperty('--font-sans')
  }

  if (settings.fontTerminal) {
    root.style.setProperty('--font-mono', `'${settings.fontTerminal}', monospace`)
  } else {
    root.style.removeProperty('--font-mono')
  }
}
