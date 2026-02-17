import type { ActionMessage } from '../api/types'

interface ActionButtonsProps {
  actions: ActionMessage[]
  onClick: (action: ActionMessage) => void
}

export default function ActionButtons({ actions, onClick }: ActionButtonsProps) {
  if (actions.length === 0) {
    return null
  }

  return (
    <div className="prompt-actions active">
      {actions.map((action) => {
        const isDanger = action.keys === '\u0003' || action.label.toLowerCase().includes('ctrl')
        return (
          <button
            key={`${action.label}-${action.keys}`}
            className={`action-btn ${isDanger ? 'danger' : ''}`.trim()}
            onClick={() => onClick(action)}
            type="button"
          >
            {action.label}
          </button>
        )
      })}
    </div>
  )
}
