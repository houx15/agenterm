import { useEffect, useMemo, useState } from 'react'
import type { ActionMessage, OutputMessage, ServerMessage } from '../api/types'
import ActionButtons from '../components/ActionButtons'
import ChatMessage, { type SessionMessage } from '../components/ChatMessage'
import Terminal from '../components/Terminal'
import { useAppContext } from '../App'

type ViewMode = 'stream' | 'raw'

export default function Sessions() {
  const app = useAppContext()
  const [streamMessages, setStreamMessages] = useState<Record<string, SessionMessage[]>>({})
  const [rawBuffers, setRawBuffers] = useState<Record<string, string>>({})
  const [viewMode, setViewMode] = useState<Record<string, ViewMode>>({})
  const [inputValue, setInputValue] = useState('')

  const mode = app.activeWindow ? (viewMode[app.activeWindow] ?? 'raw') : 'stream'
  const messages = app.activeWindow ? (streamMessages[app.activeWindow] ?? []) : []
  const rawHistory = app.activeWindow ? (rawBuffers[app.activeWindow] ?? '') : ''

  useEffect(() => {
    if (!app.lastMessage) {
      return
    }

    handleServerMessage(app.lastMessage)
  }, [app.lastMessage])

  const latestActions = useMemo(() => {
    if (messages.length === 0) {
      return []
    }
    return messages[messages.length - 1].actions ?? []
  }, [messages])

  const appendUserEcho = (text: string) => {
    if (!app.activeWindow) {
      return
    }

    setStreamMessages((prev) => {
      const list = prev[app.activeWindow!] ?? []
      return {
        ...prev,
        [app.activeWindow!]: [
          ...list,
          { text, className: 'input', isUser: true, timestamp: Date.now() },
        ],
      }
    })
  }

  const sendInput = (text: string, showBubble = true) => {
    if (!app.activeWindow || !text) {
      return
    }

    if (mode === 'raw') {
      app.send({ type: 'terminal_input', window: app.activeWindow, keys: text })
      return
    }

    if (app.send({ type: 'input', window: app.activeWindow, keys: text })) {
      const display = text.replace(/\n$/, '')
      if (display && showBubble && text !== '\u0003') {
        appendUserEcho(display)
      }
    }
  }

  const sendAction = (action: ActionMessage) => {
    sendInput(action.keys, false)
    appendUserEcho(`[${action.label}]`)
  }

  function handleServerMessage(message: ServerMessage) {
    switch (message.type) {
      case 'terminal_data': {
        setRawBuffers((prev) => {
          const current = prev[message.window] ?? ''
          const next = current + (message.text ?? '')
          return {
            ...prev,
            [message.window]: next.length > 300000 ? next.slice(next.length - 300000) : next,
          }
        })
        return
      }
      case 'output': {
        const data = message as OutputMessage
        const windowID = data.window || app.activeWindow
        if (!windowID) {
          return
        }
        setStreamMessages((prev) => {
          const list = prev[windowID] ?? []
          return {
            ...prev,
            [windowID]: [
              ...list,
              {
                text: data.text,
                className: data.class ?? 'output',
                actions: data.actions,
                timestamp: Date.now(),
              },
            ],
          }
        })
        return
      }
      default:
        return
    }
  }

  return (
    <section className="sessions-grid">
      <aside className="sessions-panel">
        <h3>Active Sessions</h3>
        {app.windows.map((win) => (
          <button
            key={win.id}
            className={`session-row ${app.activeWindow === win.id ? 'active' : ''}`.trim()}
            onClick={() => app.setActiveWindow(win.id)}
            type="button"
          >
            <span>{win.name}</span>
            <small>{win.status}</small>
          </button>
        ))}
      </aside>

      <section className="viewer-panel">
        <div className="viewer-toolbar">
          <strong>{app.activeWindow ?? 'Select a session'}</strong>
          <button
            className="secondary-btn"
            disabled={!app.activeWindow}
            onClick={() => {
              if (!app.activeWindow) {
                return
              }
              const next: ViewMode = mode === 'raw' ? 'stream' : 'raw'
              setViewMode((prev) => ({ ...prev, [app.activeWindow!]: next }))
            }}
            type="button"
          >
            {mode}
          </button>
        </div>

        {!app.activeWindow && <div className="empty-view">Select a session to start</div>}

        {app.activeWindow && mode === 'raw' && (
          <Terminal
            sessionId={app.activeWindow}
            history={rawHistory}
            onInput={(keys) => app.send({ type: 'terminal_input', window: app.activeWindow!, keys })}
            onResize={(cols, rows) => app.send({ type: 'terminal_resize', window: app.activeWindow!, cols, rows })}
          />
        )}

        {app.activeWindow && mode === 'stream' && (
          <>
            <div className="messages-wrap">
              {messages.length === 0 && <div className="empty-view">No output yet</div>}
              {messages.map((message, idx) => (
                <ChatMessage key={`${idx}-${message.timestamp}`} message={message} />
              ))}
            </div>
            <ActionButtons actions={latestActions} onClick={sendAction} />
          </>
        )}

        <div className="input-row">
          <textarea
            value={inputValue}
            onChange={(event) => setInputValue(event.target.value)}
            onKeyDown={(event) => {
              if (event.key === 'Tab') {
                event.preventDefault()
                sendInput('\t', false)
              }
              if (event.key === 'Enter' && !event.shiftKey) {
                event.preventDefault()
                if (inputValue.trim()) {
                  sendInput(`${inputValue}\n`)
                  setInputValue('')
                }
              }
            }}
            placeholder="Type command..."
          />
          <button
            className="primary-btn"
            onClick={() => {
              if (!inputValue.trim()) {
                return
              }
              sendInput(`${inputValue}\n`)
              setInputValue('')
            }}
            type="button"
          >
            Send
          </button>
        </div>
      </section>
    </section>
  )
}
