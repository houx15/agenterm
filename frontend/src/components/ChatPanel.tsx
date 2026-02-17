import { useEffect, useMemo, useRef, useState } from 'react'
import ChatMessage, { type MessageTaskLink, type SessionMessage } from './ChatMessage'

interface ChatPanelProps {
  messages: SessionMessage[]
  taskLinks: MessageTaskLink[]
  isStreaming: boolean
  connectionStatus: 'connected' | 'connecting' | 'disconnected'
  onSend: (message: string) => boolean
  onTaskClick: (taskID: string) => void
}

export default function ChatPanel({
  messages,
  taskLinks,
  isStreaming,
  connectionStatus,
  onSend,
  onTaskClick,
}: ChatPanelProps) {
  const [inputValue, setInputValue] = useState('')
  const endRef = useRef<HTMLDivElement | null>(null)

  const sendCurrent = () => {
    const text = inputValue.trim()
    if (!text) {
      return
    }
    const sent = onSend(text)
    if (sent) {
      setInputValue('')
    }
  }

  const resolvedMessages = useMemo(
    () =>
      messages.map((message) => ({
        ...message,
        taskLinks: message.taskLinks ?? taskLinks,
      })),
    [messages, taskLinks],
  )

  useEffect(() => {
    endRef.current?.scrollIntoView({ behavior: 'smooth', block: 'end' })
  }, [resolvedMessages, isStreaming])

  return (
    <section className="pm-chat-panel">
      <div className="pm-panel-header">
        <h3>Chat</h3>
        <small>{connectionStatus}</small>
      </div>

      <div className="pm-chat-messages">
        {resolvedMessages.length === 0 && <div className="empty-view">Ask the PM to plan or report progress.</div>}

        {resolvedMessages.map((message, idx) => (
          <ChatMessage
            key={`${message.id ?? 'message'}-${message.timestamp}-${idx}`}
            message={message}
            variant="pm"
            onActionClick={onSend}
            onTaskClick={onTaskClick}
          />
        ))}

        {isStreaming && <div className="pm-streaming-indicator">PM is working...</div>}

        <div ref={endRef} />
      </div>

      <div className="pm-chat-input-row">
        <textarea
          value={inputValue}
          onChange={(event) => setInputValue(event.target.value)}
          onKeyDown={(event) => {
            if (event.key === 'Enter' && !event.shiftKey) {
              event.preventDefault()
              sendCurrent()
            }
          }}
          placeholder="Ask the orchestrator..."
        />

        <div className="pm-chat-input-actions">
          <button className="secondary-btn" type="button" aria-label="Attach context" title="Attach context">
            attach
          </button>
          <button className="primary-btn" onClick={sendCurrent} type="button" disabled={!inputValue.trim()}>
            Send
          </button>
        </div>
      </div>
    </section>
  )
}
