import { useState } from 'react'
import { createAgent, createPermissionTemplate } from '../api/client'
import { CheckCircle, ChevronRight } from './Lucide'

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

interface AgentDraft {
  id: string
  name: string
  command: string
  maxParallel: number
  roles: string[]
  enabled: boolean
}

type PermissionLevel = 'standard' | 'strict' | 'permissive'

interface PermissionChoice {
  agentId: string
  level: PermissionLevel
}

interface OnboardingWizardProps {
  onComplete: () => void
}

// ---------------------------------------------------------------------------
// Defaults
// ---------------------------------------------------------------------------

const DEFAULT_AGENTS: AgentDraft[] = [
  { id: 'claude-code', name: 'Claude Code', command: 'claude --dangerously-skip-permissions', maxParallel: 2, roles: ['build', 'review', 'plan'], enabled: true },
  { id: 'codex', name: 'Codex', command: 'codex -a full-auto', maxParallel: 2, roles: ['build'], enabled: false },
  { id: 'kimi', name: 'Kimi CLI', command: 'kimi --yolo', maxParallel: 1, roles: ['build'], enabled: false },
  { id: 'gemini', name: 'Gemini CLI', command: 'gemini', maxParallel: 1, roles: ['build'], enabled: false },
]

const PERMISSION_TEMPLATES: Record<string, Record<PermissionLevel, { allow: string[]; deny: string[] }>> = {
  'claude-code': {
    standard: { allow: ['git *', 'npm *', 'go *', 'make *', 'ls *', 'cat *', 'grep *'], deny: ['rm -rf *', 'sudo *'] },
    strict: { allow: ['git status', 'git diff', 'ls *', 'cat *'], deny: ['rm *', 'sudo *', 'chmod *', 'git push *'] },
    permissive: { allow: ['*'], deny: ['sudo *'] },
  },
  codex: {
    standard: { allow: ['git *', 'npm *', 'go *', 'make *', 'ls *', 'cat *'], deny: ['rm -rf *', 'sudo *'] },
    strict: { allow: ['git status', 'git diff', 'ls *', 'cat *'], deny: ['rm *', 'sudo *', 'chmod *'] },
    permissive: { allow: ['*'], deny: ['sudo *'] },
  },
  kimi: {
    standard: { allow: ['git *', 'npm *', 'ls *', 'cat *'], deny: ['rm -rf *', 'sudo *'] },
    strict: { allow: ['git status', 'ls *', 'cat *'], deny: ['rm *', 'sudo *'] },
    permissive: { allow: ['*'], deny: ['sudo *'] },
  },
  gemini: {
    standard: { allow: ['git *', 'npm *', 'ls *', 'cat *'], deny: ['rm -rf *', 'sudo *'] },
    strict: { allow: ['git status', 'ls *', 'cat *'], deny: ['rm *', 'sudo *'] },
    permissive: { allow: ['*'], deny: ['sudo *'] },
  },
}

const LEVEL_LABELS: Record<PermissionLevel, { label: string; desc: string }> = {
  standard: { label: 'Standard', desc: 'Recommended — covers common dev tools' },
  strict: { label: 'Strict', desc: 'Read-only tools, no destructive operations' },
  permissive: { label: 'Permissive', desc: 'Allow everything except sudo' },
}

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

export default function OnboardingWizard({ onComplete }: OnboardingWizardProps) {
  const [step, setStep] = useState(1)
  const [agents, setAgents] = useState<AgentDraft[]>(DEFAULT_AGENTS)
  const [permissions, setPermissions] = useState<PermissionChoice[]>([])
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  const enabledAgents = agents.filter((a) => a.enabled)

  // Initialize permission choices when moving to step 2
  const goToStep2 = () => {
    const choices = enabledAgents.map((a) => ({
      agentId: a.id,
      level: 'standard' as PermissionLevel,
    }))
    setPermissions(choices)
    setStep(2)
  }

  const updateAgent = (id: string, updates: Partial<AgentDraft>) => {
    setAgents((prev) => prev.map((a) => (a.id === id ? { ...a, ...updates } : a)))
  }

  const updatePermission = (agentId: string, level: PermissionLevel) => {
    setPermissions((prev) => prev.map((p) => (p.agentId === agentId ? { ...p, level } : p)))
  }

  const getTemplateForAgent = (agentId: string, level: PermissionLevel) => {
    const agentTemplates = PERMISSION_TEMPLATES[agentId]
    if (agentTemplates) return agentTemplates[level]
    // Fallback to claude-code templates
    return PERMISSION_TEMPLATES['claude-code'][level]
  }

  const finish = async () => {
    setBusy(true)
    setError('')
    try {
      // Create enabled agents
      for (const agent of enabledAgents) {
        await createAgent({
          id: agent.id,
          name: agent.name,
          command: agent.command,
          max_parallel_agents: agent.maxParallel,
          capabilities: agent.roles,
          languages: [],
          cost_tier: 'medium',
          speed_tier: 'medium',
          supports_session_resume: false,
          supports_headless: false,
        })
      }

      // Create permission templates
      for (const perm of permissions) {
        const template = getTemplateForAgent(perm.agentId, perm.level)
        await createPermissionTemplate({
          agent_type: perm.agentId,
          name: `${perm.level} (${perm.agentId})`,
          config: JSON.stringify(template),
        })
      }

      onComplete()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Setup failed')
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-bg-primary/95">
      <div className="w-full max-w-2xl mx-4 rounded-lg border border-border bg-bg-secondary p-8">
        {/* Progress indicator */}
        <div className="flex items-center justify-center gap-2 mb-8">
          {[1, 2, 3].map((s) => (
            <div key={s} className="flex items-center gap-2">
              <div
                className={`w-8 h-8 rounded-full flex items-center justify-center text-sm font-medium ${
                  s < step
                    ? 'bg-accent text-white'
                    : s === step
                      ? 'bg-accent text-white'
                      : 'bg-bg-tertiary text-text-secondary'
                }`}
              >
                {s < step ? <CheckCircle size={16} /> : s}
              </div>
              {s < 3 && (
                <div className={`w-12 h-0.5 ${s < step ? 'bg-accent' : 'bg-bg-tertiary'}`} />
              )}
            </div>
          ))}
        </div>

        {/* Step 1: Set Up Your AI Team */}
        {step === 1 && (
          <div>
            <h2 className="text-xl font-semibold text-text-primary mb-2">Set Up Your AI Team</h2>
            <p className="text-sm text-text-secondary mb-6">
              Select and configure the CLI agents you want to use. You can edit commands, parallel slots, and roles.
            </p>

            <div className="space-y-3">
              {agents.map((agent) => (
                <div
                  key={agent.id}
                  className={`rounded border p-4 transition-colors ${
                    agent.enabled
                      ? 'border-accent/50 bg-bg-tertiary'
                      : 'border-border bg-bg-primary/50'
                  }`}
                >
                  <div className="flex items-center gap-3 mb-3">
                    <input
                      type="checkbox"
                      checked={agent.enabled}
                      onChange={(e) => updateAgent(agent.id, { enabled: e.target.checked })}
                      className="w-4 h-4 accent-accent"
                    />
                    <span className="font-medium text-text-primary">{agent.name}</span>
                    <div className="ml-auto flex gap-1">
                      {agent.roles.map((role) => (
                        <span
                          key={role}
                          className="px-2 py-0.5 text-xs rounded bg-accent/20 text-accent"
                        >
                          {role}
                        </span>
                      ))}
                    </div>
                  </div>

                  {agent.enabled && (
                    <div className="grid grid-cols-[1fr_auto] gap-3 ml-7">
                      <div>
                        <label className="text-xs text-text-secondary block mb-1">Command</label>
                        <input
                          value={agent.command}
                          onChange={(e) => updateAgent(agent.id, { command: e.target.value })}
                          className="w-full rounded border border-border bg-bg-primary px-3 py-1.5 text-sm text-text-primary font-mono focus:border-accent focus:outline-none"
                        />
                      </div>
                      <div>
                        <label className="text-xs text-text-secondary block mb-1">Parallel</label>
                        <input
                          type="number"
                          min={1}
                          max={8}
                          value={agent.maxParallel}
                          onChange={(e) =>
                            updateAgent(agent.id, {
                              maxParallel: Math.max(1, Math.min(8, Number(e.target.value) || 1)),
                            })
                          }
                          className="w-20 rounded border border-border bg-bg-primary px-3 py-1.5 text-sm text-text-primary text-center focus:border-accent focus:outline-none"
                        />
                      </div>
                    </div>
                  )}
                </div>
              ))}
            </div>

            <div className="flex justify-end mt-6">
              <button
                onClick={goToStep2}
                disabled={enabledAgents.length === 0}
                className="flex items-center gap-2 rounded bg-accent px-6 py-2.5 text-sm font-medium text-white hover:bg-accent/80 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
              >
                Next <ChevronRight size={16} />
              </button>
            </div>
          </div>
        )}

        {/* Step 2: Permission Templates */}
        {step === 2 && (
          <div>
            <h2 className="text-xl font-semibold text-text-primary mb-2">Permission Templates</h2>
            <p className="text-sm text-text-secondary mb-6">
              Choose a permission level for each agent. This controls what commands the agent is allowed to run.
            </p>

            <div className="space-y-6">
              {permissions.map((perm) => {
                const agent = agents.find((a) => a.id === perm.agentId)
                if (!agent) return null
                const template = getTemplateForAgent(perm.agentId, perm.level)

                return (
                  <div key={perm.agentId} className="rounded border border-border bg-bg-tertiary p-4">
                    <h3 className="text-sm font-medium text-text-primary mb-3">{agent.name}</h3>
                    <div className="flex gap-2 mb-3">
                      {(['standard', 'strict', 'permissive'] as PermissionLevel[]).map((level) => (
                        <button
                          key={level}
                          onClick={() => updatePermission(perm.agentId, level)}
                          className={`px-3 py-1.5 text-xs rounded transition-colors ${
                            perm.level === level
                              ? 'bg-accent text-white'
                              : 'bg-bg-primary text-text-secondary hover:text-text-primary hover:bg-bg-primary/80'
                          }`}
                        >
                          {LEVEL_LABELS[level].label}
                        </button>
                      ))}
                    </div>
                    <p className="text-xs text-text-secondary mb-2">
                      {LEVEL_LABELS[perm.level].desc}
                    </p>
                    <div className="rounded bg-bg-primary p-3 text-xs font-mono">
                      <div className="text-status-working mb-1">
                        Allow: {template.allow.join(', ')}
                      </div>
                      <div className="text-status-error">
                        Deny: {template.deny.join(', ')}
                      </div>
                    </div>
                  </div>
                )
              })}
            </div>

            <div className="flex justify-between mt-6">
              <button
                onClick={() => setStep(1)}
                className="rounded px-6 py-2.5 text-sm text-text-secondary hover:text-text-primary hover:bg-bg-tertiary transition-colors"
              >
                Back
              </button>
              <button
                onClick={() => setStep(3)}
                className="flex items-center gap-2 rounded bg-accent px-6 py-2.5 text-sm font-medium text-white hover:bg-accent/80 transition-colors"
              >
                Next <ChevronRight size={16} />
              </button>
            </div>
          </div>
        )}

        {/* Step 3: Done */}
        {step === 3 && (
          <div className="text-center">
            <div className="flex justify-center mb-4">
              <div className="w-16 h-16 rounded-full bg-accent/20 flex items-center justify-center">
                <CheckCircle size={32} className="text-accent" />
              </div>
            </div>
            <h2 className="text-xl font-semibold text-text-primary mb-2">Ready to Go</h2>
            <p className="text-sm text-text-secondary mb-6">
              Here is a summary of your setup:
            </p>

            <div className="rounded border border-border bg-bg-tertiary p-4 mb-6 text-left">
              <div className="text-sm text-text-primary mb-2">
                <span className="text-accent font-medium">{enabledAgents.length}</span> agent{enabledAgents.length !== 1 ? 's' : ''} configured
              </div>
              <ul className="text-xs text-text-secondary space-y-1 ml-4 list-disc">
                {enabledAgents.map((a) => (
                  <li key={a.id}>
                    {a.name} — {a.maxParallel} slot{a.maxParallel !== 1 ? 's' : ''}, roles: {a.roles.join(', ')}
                  </li>
                ))}
              </ul>
              <div className="text-sm text-text-primary mt-3 mb-2">
                <span className="text-accent font-medium">{permissions.length}</span> permission template{permissions.length !== 1 ? 's' : ''}
              </div>
              <ul className="text-xs text-text-secondary space-y-1 ml-4 list-disc">
                {permissions.map((p) => {
                  const agent = agents.find((a) => a.id === p.agentId)
                  return (
                    <li key={p.agentId}>
                      {agent?.name ?? p.agentId} — {LEVEL_LABELS[p.level].label}
                    </li>
                  )
                })}
              </ul>
            </div>

            {error && (
              <div className="rounded border border-status-error/50 bg-status-error/10 text-status-error text-sm p-3 mb-4">
                {error}
              </div>
            )}

            <div className="flex justify-between">
              <button
                onClick={() => setStep(2)}
                disabled={busy}
                className="rounded px-6 py-2.5 text-sm text-text-secondary hover:text-text-primary hover:bg-bg-tertiary transition-colors disabled:opacity-50"
              >
                Back
              </button>
              <button
                onClick={() => void finish()}
                disabled={busy}
                className="rounded bg-accent px-8 py-2.5 text-sm font-medium text-white hover:bg-accent/80 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
              >
                {busy ? 'Setting up...' : 'Get Started'}
              </button>
            </div>
          </div>
        )}
      </div>
    </div>
  )
}
