import { useEffect } from 'react'

interface ShortcutActions {
  switchProject: (index: number) => void
  togglePanel: () => void
  focusActiveTerminal: () => void
  focusChatInput: () => void
}

export function useKeyboardShortcuts(actions: ShortcutActions) {
  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      const meta = e.metaKey || e.ctrlKey

      // Cmd+1..9 — switch project
      if (meta && !e.shiftKey && e.key >= '1' && e.key <= '9') {
        e.preventDefault()
        actions.switchProject(parseInt(e.key, 10) - 1)
        return
      }

      // Cmd+Shift+A — toggle orchestrator panel
      if (meta && e.shiftKey && e.key.toLowerCase() === 'a') {
        e.preventDefault()
        actions.togglePanel()
        return
      }

      // Cmd+Shift+T — focus active terminal
      if (meta && e.shiftKey && e.key.toLowerCase() === 't') {
        e.preventDefault()
        actions.focusActiveTerminal()
        return
      }

      // Cmd+K — focus chat input
      if (meta && e.key.toLowerCase() === 'k') {
        e.preventDefault()
        actions.focusChatInput()
        return
      }
    }

    window.addEventListener('keydown', handler)
    return () => window.removeEventListener('keydown', handler)
  }, [actions])
}
