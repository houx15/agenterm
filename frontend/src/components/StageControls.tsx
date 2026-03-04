import { useState } from 'react'
import { transitionStage } from '../api/client'
import { ArrowRight, CheckCircle } from './Lucide'

interface StageControlsProps {
  requirementID: string
  currentStage: string
  onTransition: () => void
}

interface TransitionConfig {
  label: string
  nextStage: string
  variant: 'accent' | 'success'
}

function getTransition(stage: string): TransitionConfig | null {
  switch (stage) {
    case 'building':
      return { label: 'Start Review', nextStage: 'review', variant: 'accent' }
    case 'reviewing':
      return { label: 'Merge All', nextStage: 'merge', variant: 'accent' }
    case 'merge':
      return { label: "I'll Test Now", nextStage: 'test', variant: 'accent' }
    case 'testing':
      return { label: 'Mark Done', nextStage: 'done', variant: 'success' }
    default:
      return null
  }
}

export default function StageControls({ requirementID, currentStage, onTransition }: StageControlsProps) {
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  const transition = getTransition(currentStage)
  if (!transition) return null

  const handleClick = async () => {
    setLoading(true)
    setError('')
    try {
      await transitionStage(requirementID, transition.nextStage)
      onTransition()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Transition failed')
    } finally {
      setLoading(false)
    }
  }

  const Icon = transition.variant === 'success' ? CheckCircle : ArrowRight

  return (
    <div className="flex items-center gap-3 px-4 py-2 border-b border-border bg-bg-secondary shrink-0">
      <button
        className={`flex items-center gap-2 rounded px-4 py-2 text-sm font-medium text-white transition-colors disabled:opacity-50 disabled:cursor-not-allowed ${
          transition.variant === 'success'
            ? 'bg-status-working hover:bg-status-working/80'
            : 'bg-accent hover:bg-accent/80'
        }`}
        onClick={() => void handleClick()}
        disabled={loading}
        type="button"
      >
        <Icon size={14} />
        {loading ? 'Processing...' : transition.label}
      </button>
      {error && <span className="text-xs text-status-error">{error}</span>}
    </div>
  )
}
