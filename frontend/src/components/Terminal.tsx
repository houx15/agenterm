import { FitAddon } from '@xterm/addon-fit'
import { Terminal as XTerm } from '@xterm/xterm'
import '@xterm/xterm/css/xterm.css'
import { useEffect, useRef } from 'react'

interface TerminalProps {
  sessionId: string
  history: string
  onInput: (keys: string) => void
  onResize: (cols: number, rows: number) => void
}

export default function Terminal({ sessionId, history, onInput, onResize }: TerminalProps) {
  const rootRef = useRef<HTMLDivElement | null>(null)
  const terminalRef = useRef<XTerm | null>(null)
  const fitAddonRef = useRef<FitAddon | null>(null)
  const currentSessionRef = useRef<string>('')

  useEffect(() => {
    if (!rootRef.current || terminalRef.current) {
      return
    }

    const term = new XTerm({
      convertEol: false,
      cursorBlink: true,
      fontSize: 13,
      theme: {
        background: '#0a0f1a',
        foreground: '#d1d5db',
      },
    })

    const fit = new FitAddon()
    term.loadAddon(fit)
    term.open(rootRef.current)
    fit.fit()

    term.onData((data) => {
      onInput(data)
    })

    terminalRef.current = term
    fitAddonRef.current = fit

    const observer = new ResizeObserver(() => {
      fit.fit()
      onResize(term.cols, term.rows)
    })
    observer.observe(rootRef.current)

    onResize(term.cols, term.rows)

    return () => {
      observer.disconnect()
      term.dispose()
      terminalRef.current = null
      fitAddonRef.current = null
    }
  }, [onInput, onResize])

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
    }

    fit.fit()
    onResize(term.cols, term.rows)
  }, [history, onResize, sessionId])

  return <div className="terminal" ref={rootRef} />
}
