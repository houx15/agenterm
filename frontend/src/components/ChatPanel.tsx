import { useEffect, useMemo, useRef, useState } from 'react'
import ChatMessage, { type MessageTaskLink, type SessionMessage } from './ChatMessage'
import { useSpeechToText } from '../hooks/useSpeechToText'
import { Activity } from './Lucide'

interface ChatPanelProps {
  messages: SessionMessage[]
  taskLinks: MessageTaskLink[]
  isStreaming: boolean
  connectionStatus: 'connected' | 'connecting' | 'disconnected'
  onSend: (message: string) => boolean
  onTaskClick: (taskID: string) => void
  onReportProgress?: () => void
  isFetchingReport?: boolean
}

export default function ChatPanel({
  messages,
  taskLinks,
  isStreaming,
  connectionStatus,
  onSend,
  onTaskClick,
  onReportProgress,
  isFetchingReport = false,
}: ChatPanelProps) {
  const [inputValue, setInputValue] = useState('')
  const [actionDecisions, setActionDecisions] = useState<Record<string, string>>({})
  const listRef = useRef<HTMLDivElement | null>(null)
  const speech = useSpeechToText({
    onTranscript: (text) => {
      setInputValue((prev) => (prev.trim() ? `${prev.trim()} ${text}` : text))
    },
  })

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
      messages.map((message) => {
        const decision = message.id ? actionDecisions[message.id] : ''
        if (!decision) {
          return {
            ...message,
            taskLinks: message.taskLinks ?? taskLinks,
          }
        }
        return {
          ...message,
          text: `${message.text}\n\n${decision}`,
          confirmationOptions: [],
          taskLinks: message.taskLinks ?? taskLinks,
        }
      }),
    [actionDecisions, messages, taskLinks],
  )

  const handleActionClick = (value: string, label: string, messageID?: string) => {
    const sent = onSend(value)
    if (!sent || !messageID) {
      return
    }
    const lower = label.trim().toLowerCase()
    let note = 'You responded to this confirmation.'
    if (lower.includes('confirm')) {
      note = 'You confirmed this operation.'
    } else if (lower.includes('cancel')) {
      note = 'You rejected this operation.'
    } else if (lower.includes('modify')) {
      note = 'You requested plan modification.'
    }
    setActionDecisions((prev) => ({ ...prev, [messageID]: note }))
  }

  useEffect(() => {
    const node = listRef.current
    if (!node) {
      return
    }
    const distanceFromBottom = node.scrollHeight - node.scrollTop - node.clientHeight
    if (distanceFromBottom > 120 && !isStreaming) {
      return
    }
    node.scrollTo({ top: node.scrollHeight, behavior: 'smooth' })
  }, [resolvedMessages, isStreaming])

  return (
    <section className="pm-chat-panel">
      <div className="pm-panel-header">
        <h3>Chat</h3>
        <small>{connectionStatus}</small>
      </div>

      <div className="pm-chat-messages" ref={listRef}>
        {resolvedMessages.length === 0 && <div className="empty-view">Ask the PM to plan or report progress.</div>}

        {resolvedMessages.map((message, idx) => (
          <ChatMessage
            key={`${message.id ?? 'message'}-${message.timestamp}-${idx}`}
            message={message}
            variant="pm"
            onActionClick={handleActionClick}
            onTaskClick={onTaskClick}
          />
        ))}

        {isStreaming && <div className="pm-streaming-indicator">PM is working...</div>}

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
          {onReportProgress && (
            <button
              className="secondary-btn"
              type="button"
              onClick={onReportProgress}
              disabled={isFetchingReport || connectionStatus !== 'connected'}
              title="Get current orchestrator progress summary"
            >
              <Activity size={14} />
              <span>{isFetchingReport ? 'Reporting…' : 'Report Progress'}</span>
            </button>
          )}
          <button className="secondary-btn icon-only-btn" type="button" aria-label="Attach context" title="Attach context">
            <svg viewBox="0 0 24 24" aria-hidden="true" focusable="false">
              <path
                d="M8.5 12.5l6.8-6.8a3 3 0 114.2 4.2l-8.2 8.2a5 5 0 11-7.1-7.1L13 2.4"
                fill="none"
                stroke="currentColor"
                strokeWidth="1.8"
                strokeLinecap="round"
                strokeLinejoin="round"
              />
            </svg>
            <span className="sr-only">Attach context</span>
          </button>
          <button
            className={`secondary-btn icon-only-btn ${speech.isRecording ? 'recording' : ''}`.trim()}
            type="button"
            aria-label={speech.isRecording ? 'Stop recording' : 'Start recording'}
            title={speech.isRecording ? 'Stop recording' : 'Start recording'}
            onClick={speech.toggleRecording}
            disabled={!speech.supported || speech.isTranscribing}
          >
            {speech.isRecording ? (
              <svg viewBox="0 0 24 24" aria-hidden="true" focusable="false">
                <rect x="7" y="7" width="10" height="10" rx="2" fill="currentColor" />
              </svg>
            ) : (
              <svg viewBox="0 0 24 24" aria-hidden="true" focusable="false">
                <path
                  d="M12 3a3 3 0 00-3 3v6a3 3 0 006 0V6a3 3 0 00-3-3zm6 9a1 1 0 10-2 0 4 4 0 11-8 0 1 1 0 10-2 0 6 6 0 005 5.9V21H9a1 1 0 000 2h6a1 1 0 100-2h-2v-3.1A6 6 0 0018 12z"
                  fill="currentColor"
                />
              </svg>
            )}
            <span className="sr-only">{speech.isRecording ? 'Stop recording' : 'Start recording'}</span>
          </button>
          <button className="primary-btn" onClick={sendCurrent} type="button" disabled={!inputValue.trim()}>
            Send
          </button>
        </div>
        {(speech.isTranscribing || speech.error) && (
          <div className={`pm-chat-speech-status ${speech.error ? 'error' : ''}`.trim()}>
            {speech.isTranscribing ? 'Transcribing speech...' : speech.error}
          </div>
        )}
      </div>
    </section>
  )
}
