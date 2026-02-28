import { FitAddon } from '@xterm/addon-fit'
import { Terminal as XTerm, type ITheme } from '@xterm/xterm'
import '@xterm/xterm/css/xterm.css'
import { useEffect, useRef } from 'react'

interface TerminalProps {
  sessionId: string
  history: string
  onInput: (keys: string) => void
  onResize: (cols: number, rows: number) => void
  terminalFontSize?: number
  terminalFontFamily?: string
  themeKey?: string
}

function readCSSVar(name: string): string {
  return getComputedStyle(document.documentElement).getPropertyValue(name).trim()
}

function buildXtermTheme(): ITheme {
  return {
    background: readCSSVar('--terminal-bg') || '#0a0f1a',
    foreground: readCSSVar('--terminal-fg') || '#d1d5db',
    cursor: readCSSVar('--terminal-cursor') || '#d1d5db',
    selectionBackground: readCSSVar('--terminal-selection') || undefined,
    black: readCSSVar('--terminal-black') || undefined,
    red: readCSSVar('--terminal-red') || undefined,
    green: readCSSVar('--terminal-green') || undefined,
    yellow: readCSSVar('--terminal-yellow') || undefined,
    blue: readCSSVar('--terminal-blue') || undefined,
    magenta: readCSSVar('--terminal-magenta') || undefined,
    cyan: readCSSVar('--terminal-cyan') || undefined,
    white: readCSSVar('--terminal-white') || undefined,
    brightBlack: readCSSVar('--terminal-bright-black') || undefined,
    brightRed: readCSSVar('--terminal-bright-red') || undefined,
    brightGreen: readCSSVar('--terminal-bright-green') || undefined,
    brightYellow: readCSSVar('--terminal-bright-yellow') || undefined,
    brightBlue: readCSSVar('--terminal-bright-blue') || undefined,
    brightMagenta: readCSSVar('--terminal-bright-magenta') || undefined,
    brightCyan: readCSSVar('--terminal-bright-cyan') || undefined,
    brightWhite: readCSSVar('--terminal-bright-white') || undefined,
  }
}

export default function Terminal({
  sessionId,
  history,
  onInput,
  onResize,
  terminalFontSize = 13,
  terminalFontFamily,
  themeKey,
}: TerminalProps) {
  const rootRef = useRef<HTMLDivElement | null>(null)
  const terminalRef = useRef<XTerm | null>(null)
  const fitAddonRef = useRef<FitAddon | null>(null)
  const currentSessionRef = useRef<string>('')
  const lastRenderedHistoryRef = useRef<string>('')
  const onInputRef = useRef(onInput)
  const onResizeRef = useRef(onResize)

  useEffect(() => {
    onInputRef.current = onInput
  }, [onInput])

  useEffect(() => {
    onResizeRef.current = onResize
  }, [onResize])

  useEffect(() => {
    if (!rootRef.current || terminalRef.current) {
      return
    }

    const term = new XTerm({
      convertEol: false,
      cursorBlink: true,
      fontSize: terminalFontSize,
      fontFamily: terminalFontFamily || undefined,
      theme: buildXtermTheme(),
    })

    const fit = new FitAddon()
    term.loadAddon(fit)
    term.open(rootRef.current)
    fit.fit()

    term.onData((data) => {
      onInputRef.current(data)
    })

    terminalRef.current = term
    fitAddonRef.current = fit

    const observer = new ResizeObserver(() => {
      fit.fit()
      onResizeRef.current(term.cols, term.rows)
    })
    observer.observe(rootRef.current)

    onResizeRef.current(term.cols, term.rows)

    return () => {
      observer.disconnect()
      term.dispose()
      terminalRef.current = null
      fitAddonRef.current = null
    }
  }, [])

  // Re-apply theme/font when props change
  useEffect(() => {
    const term = terminalRef.current
    if (!term) return
    term.options.theme = buildXtermTheme()
    term.options.fontSize = terminalFontSize
    if (terminalFontFamily) {
      term.options.fontFamily = terminalFontFamily
    }
    fitAddonRef.current?.fit()
  }, [terminalFontSize, terminalFontFamily, themeKey])

  useEffect(() => {
    const term = terminalRef.current
    const fit = fitAddonRef.current
    if (!term || !fit) {
      return
    }

    if (currentSessionRef.current !== sessionId) {
      currentSessionRef.current = sessionId
      term.reset()
      if (history) {
        term.write(history)
      }
      lastRenderedHistoryRef.current = history
    } else if (history !== lastRenderedHistoryRef.current) {
      const previous = lastRenderedHistoryRef.current
      if (history.startsWith(previous)) {
        const delta = history.slice(previous.length)
        if (delta) {
          term.write(delta)
        }
      } else {
        term.reset()
        if (history) {
          term.write(history)
        }
      }
      lastRenderedHistoryRef.current = history
    }

    fit.fit()
    onResizeRef.current(term.cols, term.rows)
  }, [history, sessionId])

  return <div className="terminal" ref={rootRef} />
}
