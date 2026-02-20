import { useEffect, useMemo, useState } from 'react'
import { CircleHelp, Hammer, Map, Plus, ShieldCheck, Trash2 } from 'lucide-react'
import {
  createAgent,
  createPlaybook,
  deleteAgent,
  deletePlaybook,
  listAgents,
  listPlaybooks,
  updateAgent,
  updatePlaybook,
} from '../api/client'
import type {
  AgentConfig,
  Playbook,
  PlaybookPhase,
  PlaybookWorkflow,
  PlaybookWorkflowRole,
  PlaybookWorkflowStage,
} from '../api/types'
import { loadASRSettings, saveASRSettings } from '../settings/asr'
import Modal from '../components/Modal'

type TabKey = 'agents' | 'playbooks' | 'asr'
type WorkflowStageKey = keyof PlaybookWorkflow
type RoleTemplateKey = 'planner' | 'worker' | 'reviewer' | 'tester'

const TOOL_OPTIONS = [
  'get_project_status',
  'create_task',
  'create_worktree',
  'write_task_spec',
  'create_session',
  'wait_for_session_ready',
  'send_command',
  'read_session_output',
  'is_session_idle',
  'can_close_session',
  'close_session',
  'merge_worktree',
  'resolve_merge_conflict',
  'generate_progress_report',
]

const DEFAULT_AGENT: AgentConfig = {
  id: '',
  name: '',
  model: 'default',
  command: '',
  max_parallel_agents: 1,
  supports_orchestrator: false,
  orchestrator_provider: 'anthropic',
  orchestrator_api_key: '',
  orchestrator_api_base: '',
  capabilities: [],
  languages: [],
  cost_tier: 'medium',
  speed_tier: 'medium',
  supports_session_resume: false,
  supports_headless: false,
  notes: '',
}

function createDefaultRole(name = '', mode: PlaybookWorkflowRole['mode'] = 'worker'): PlaybookWorkflowRole {
  const normalizedMode = mode ?? 'worker'
  const defaultsByMode: Record<string, Partial<PlaybookWorkflowRole>> = {
    planner: {
      mode: 'planner',
      inputs_required: ['goal'],
      actions_allowed: ['get_project_status', 'create_task', 'write_task_spec', 'create_session'],
      outputs_contract: { type: 'plan_result', required: ['strategy', 'task_graph'] },
      gates: { requires_user_approval: true, pass_condition: 'user_plan_approved' },
      retry_policy: { max_iterations: 2, escalate_on: ['unclear_scope'] },
      completion_criteria: ['Task graph accepted by user'],
    },
    worker: {
      mode: 'worker',
      inputs_required: ['spec_path'],
      actions_allowed: ['send_command', 'read_session_output', 'is_session_idle', 'close_session'],
      outputs_contract: { type: 'work_result', required: ['commit_sha', 'summary'] },
      gates: { requires_user_approval: false, pass_condition: 'changes_compiled_and_tested' },
      retry_policy: { max_iterations: 3, escalate_on: ['no_progress'] },
      completion_criteria: ['Spec implemented and committed'],
    },
    reviewer: {
      mode: 'reviewer',
      inputs_required: ['spec_path', 'commit_sha'],
      actions_allowed: ['send_command', 'read_session_output', 'is_session_idle'],
      outputs_contract: { type: 'review_result', required: ['verdict', 'issues'] },
      gates: { requires_user_approval: false, pass_condition: "verdict == 'pass'" },
      retry_policy: { max_iterations: 3, escalate_on: ['same_issue_repeated'] },
      completion_criteria: ['Review verdict is pass'],
    },
    tester: {
      mode: 'tester',
      inputs_required: ['spec_path'],
      actions_allowed: ['send_command', 'read_session_output', 'is_session_idle', 'close_session'],
      outputs_contract: { type: 'test_result', required: ['summary', 'failed_cases'] },
      gates: { requires_user_approval: false, pass_condition: 'failed_cases == 0' },
      retry_policy: { max_iterations: 3, escalate_on: ['flaky_tests'] },
      completion_criteria: ['All required checks pass'],
    },
  }
  const defaults = defaultsByMode[normalizedMode] ?? defaultsByMode.worker
  return {
    name,
    responsibilities: '',
    allowed_agents: [],
    mode: defaults.mode ?? normalizedMode,
    inputs_required: [...(defaults.inputs_required ?? [])],
    actions_allowed: [...(defaults.actions_allowed ?? [])],
    handoff_to: [],
    completion_criteria: [...(defaults.completion_criteria ?? [])],
    outputs_contract: {
      type: defaults.outputs_contract?.type ?? '',
      required: [...(defaults.outputs_contract?.required ?? [])],
    },
    gates: {
      requires_user_approval: !!defaults.gates?.requires_user_approval,
      pass_condition: defaults.gates?.pass_condition ?? '',
    },
    retry_policy: {
      max_iterations: defaults.retry_policy?.max_iterations ?? 0,
      escalate_on: [...(defaults.retry_policy?.escalate_on ?? [])],
    },
    suggested_prompt: '',
  }
}

const DEFAULT_WORKFLOW: PlaybookWorkflow = {
  plan: { enabled: true, roles: [createDefaultRole('planner', 'planner')], stage_policy: {} },
  build: { enabled: true, roles: [createDefaultRole('implementer', 'worker')], stage_policy: {} },
  test: { enabled: true, roles: [createDefaultRole('tester', 'tester')], stage_policy: {} },
}

const DEFAULT_PLAYBOOK: Playbook = {
  id: '',
  name: '',
  description: '',
  phases: [{ name: 'Implement', agent: 'codex', role: 'implementer', description: '' }],
  workflow: DEFAULT_WORKFLOW,
}

function clampParallelAgents(value: number): number {
  if (!Number.isFinite(value)) {
    return 1
  }
  return Math.min(64, Math.max(1, Math.trunc(value)))
}

function clampNonNegative(value: number): number {
  if (!Number.isFinite(value)) {
    return 0
  }
  return Math.max(0, Math.trunc(value))
}

function parseCSV(value: string): string[] {
  return value
    .split(',')
    .map((item) => item.trim())
    .filter(Boolean)
}

function toStringArray(value: unknown): string[] {
  if (!Array.isArray(value)) {
    return []
  }
  return value.filter((item): item is string => typeof item === 'string').map((item) => item.trim()).filter(Boolean)
}

const ROLE_TEMPLATES: Record<RoleTemplateKey, Partial<PlaybookWorkflowRole>> = {
  planner: {
    mode: 'planner',
    inputs_required: ['goal'],
    actions_allowed: ['get_project_status', 'create_task', 'create_worktree', 'write_task_spec', 'create_session'],
    outputs_contract: { type: 'plan_result', required: ['strategy', 'task_graph'] },
    gates: { requires_user_approval: true, pass_condition: 'user_plan_approved' },
    retry_policy: { max_iterations: 2, escalate_on: ['unclear_scope'] },
    completion_criteria: ['Task graph accepted by user'],
  },
  worker: {
    mode: 'worker',
    inputs_required: ['spec_path'],
    actions_allowed: ['send_command', 'read_session_output', 'is_session_idle', 'close_session'],
    outputs_contract: { type: 'work_result', required: ['commit_sha', 'summary'] },
    gates: { requires_user_approval: false, pass_condition: 'changes_compiled_and_tested' },
    retry_policy: { max_iterations: 5, escalate_on: ['no_progress'] },
    completion_criteria: ['Spec implemented and committed'],
  },
  reviewer: {
    mode: 'reviewer',
    inputs_required: ['spec_path', 'commit_sha'],
    actions_allowed: ['send_command', 'read_session_output', 'is_session_idle'],
    outputs_contract: { type: 'review_result', required: ['verdict', 'issues'] },
    gates: { requires_user_approval: false, pass_condition: "verdict == 'pass'" },
    retry_policy: { max_iterations: 5, escalate_on: ['same_issue_repeated'] },
    completion_criteria: ['Review verdict is pass'],
  },
  tester: {
    mode: 'tester',
    inputs_required: ['spec_path'],
    actions_allowed: ['send_command', 'read_session_output', 'is_session_idle', 'close_session'],
    outputs_contract: { type: 'test_result', required: ['summary', 'failed_cases'] },
    gates: { requires_user_approval: false, pass_condition: 'failed_cases == 0' },
    retry_policy: { max_iterations: 3, escalate_on: ['flaky_tests'] },
    completion_criteria: ['All required checks pass'],
  },
}

function withDetailedDefaults(role: PlaybookWorkflowRole): PlaybookWorkflowRole {
  const base = createDefaultRole(role.name, role.mode)
  return {
    ...role,
    inputs_required: role.inputs_required && role.inputs_required.length > 0 ? [...role.inputs_required] : [...(base.inputs_required ?? [])],
    actions_allowed: role.actions_allowed && role.actions_allowed.length > 0 ? [...role.actions_allowed] : [...(base.actions_allowed ?? [])],
    completion_criteria:
      role.completion_criteria && role.completion_criteria.length > 0 ? [...role.completion_criteria] : [...(base.completion_criteria ?? [])],
    outputs_contract: {
      type: role.outputs_contract?.type?.trim() ? role.outputs_contract.type : base.outputs_contract?.type ?? '',
      required:
        role.outputs_contract?.required && role.outputs_contract.required.length > 0
          ? [...role.outputs_contract.required]
          : [...(base.outputs_contract?.required ?? [])],
    },
    gates: {
      requires_user_approval: role.gates?.requires_user_approval ?? base.gates?.requires_user_approval ?? false,
      pass_condition: role.gates?.pass_condition?.trim() ? role.gates.pass_condition : base.gates?.pass_condition ?? '',
    },
    retry_policy: {
      max_iterations:
        typeof role.retry_policy?.max_iterations === 'number' ? role.retry_policy.max_iterations : base.retry_policy?.max_iterations ?? 0,
      escalate_on:
        role.retry_policy?.escalate_on && role.retry_policy.escalate_on.length > 0
          ? [...role.retry_policy.escalate_on]
          : [...(base.retry_policy?.escalate_on ?? [])],
    },
  }
}

function normalizeRole(role: unknown): PlaybookWorkflowRole {
  const record = role && typeof role === 'object' && !Array.isArray(role) ? (role as Record<string, unknown>) : {}
  const modeRaw = typeof record.mode === 'string' ? record.mode.trim().toLowerCase() : ''
  const mode = modeRaw || 'worker'
  const outputsContract =
    record.outputs_contract && typeof record.outputs_contract === 'object' && !Array.isArray(record.outputs_contract)
      ? (record.outputs_contract as Record<string, unknown>)
      : {}
  const gates = record.gates && typeof record.gates === 'object' && !Array.isArray(record.gates) ? (record.gates as Record<string, unknown>) : {}
  const retryPolicy =
    record.retry_policy && typeof record.retry_policy === 'object' && !Array.isArray(record.retry_policy)
      ? (record.retry_policy as Record<string, unknown>)
      : {}
  return withDetailedDefaults({
    name: typeof record.name === 'string' ? record.name : '',
    responsibilities: typeof record.responsibilities === 'string' ? record.responsibilities : '',
    allowed_agents: toStringArray(record.allowed_agents),
    suggested_prompt: typeof record.suggested_prompt === 'string' ? record.suggested_prompt : '',
    mode,
    inputs_required: toStringArray(record.inputs_required),
    actions_allowed: toStringArray(record.actions_allowed),
    handoff_to: toStringArray(record.handoff_to),
    completion_criteria: toStringArray(record.completion_criteria),
    outputs_contract: {
      type: typeof outputsContract.type === 'string' ? outputsContract.type.trim() : '',
      required: toStringArray(outputsContract.required),
    },
    gates: {
      requires_user_approval: Boolean(gates.requires_user_approval),
      pass_condition: typeof gates.pass_condition === 'string' ? gates.pass_condition : '',
    },
    retry_policy: {
      max_iterations:
        typeof retryPolicy.max_iterations === 'number' && Number.isFinite(retryPolicy.max_iterations)
          ? Math.max(0, Math.trunc(retryPolicy.max_iterations))
          : 0,
      escalate_on: toStringArray(retryPolicy.escalate_on),
    },
  })
}

function normalizeStage(stage: unknown): PlaybookWorkflowStage {
  const record = stage && typeof stage === 'object' && !Array.isArray(stage) ? (stage as Record<string, unknown>) : {}
  const enabled = typeof record.enabled === 'boolean' ? record.enabled : false
  const roles = Array.isArray(record.roles) ? record.roles.map(normalizeRole) : []
  const stagePolicy =
    record.stage_policy && typeof record.stage_policy === 'object' && !Array.isArray(record.stage_policy)
      ? (record.stage_policy as Record<string, unknown>)
      : {}
  return {
    enabled,
    roles,
    stage_policy: {
      enter_gate: typeof stagePolicy.enter_gate === 'string' ? stagePolicy.enter_gate : '',
      exit_gate: typeof stagePolicy.exit_gate === 'string' ? stagePolicy.exit_gate : '',
      max_parallel_worktrees:
        typeof stagePolicy.max_parallel_worktrees === 'number' && Number.isFinite(stagePolicy.max_parallel_worktrees)
          ? Math.max(0, Math.trunc(stagePolicy.max_parallel_worktrees))
          : 0,
    },
  }
}

function phaseToWorkflowRole(phase: PlaybookPhase): PlaybookWorkflowRole {
  return {
    name: phase.role || phase.name,
    responsibilities: phase.description || `Run ${phase.name}`,
    allowed_agents: phase.agent ? [phase.agent] : [],
    mode: 'worker',
    inputs_required: [],
    actions_allowed: [],
    handoff_to: [],
    completion_criteria: [],
    outputs_contract: { type: '', required: [] },
    gates: { requires_user_approval: false, pass_condition: '' },
    retry_policy: { max_iterations: 0, escalate_on: [] },
    suggested_prompt: '',
  }
}

function parseAPIErrorMessage(error: unknown): string {
  if (!(error instanceof Error)) {
    return 'Failed to save playbook'
  }
  const raw = error.message.trim()
  if (!raw) {
    return 'Failed to save playbook'
  }
  try {
    const parsed = JSON.parse(raw) as unknown
    if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
      const payload = parsed as Record<string, unknown>
      if (typeof payload.error === 'string' && payload.error.trim()) {
        return payload.error.trim()
      }
    }
  } catch {
    // not JSON, fallback to raw
  }
  return raw
}

function validatePlaybookDraft(pb: Playbook, isNew: boolean): string | null {
  if (isNew && !pb.id.trim()) {
    return 'Playbook ID is required.'
  }
  if (!pb.name.trim()) {
    return 'Playbook name is required.'
  }
  const stages: WorkflowStageKey[] = ['plan', 'build', 'test']
  for (const stageKey of stages) {
    const stage = pb.workflow[stageKey]
    if (!stage.enabled) {
      continue
    }
    if (!stage.roles || stage.roles.length === 0) {
      return `${stageKey} stage is enabled but has no roles.`
    }
    for (let idx = 0; idx < stage.roles.length; idx += 1) {
      const role = stage.roles[idx]
      if (!role.name.trim()) {
        return `${stageKey} stage role ${idx + 1} is missing a role name.`
      }
    }
  }
  return null
}

function fallbackWorkflowFromPhases(phases: PlaybookPhase[]): PlaybookWorkflow {
  const workflow: PlaybookWorkflow = {
    plan: { enabled: true, roles: [], stage_policy: {} },
    build: { enabled: true, roles: [], stage_policy: {} },
    test: { enabled: true, roles: [], stage_policy: {} },
  }
  phases.forEach((phase) => {
    const lower = `${phase.name} ${phase.role}`.toLowerCase()
    const role = phaseToWorkflowRole(phase)
    if (lower.includes('plan') || lower.includes('discover') || lower.includes('architect')) {
      workflow.plan.roles.push(role)
      return
    }
    if (lower.includes('test') || lower.includes('review') || lower.includes('qa') || lower.includes('verify')) {
      workflow.test.roles.push(role)
      return
    }
    workflow.build.roles.push(role)
  })
  workflow.plan.enabled = workflow.plan.roles.length > 0
  workflow.build.enabled = workflow.build.roles.length > 0
  workflow.test.enabled = workflow.test.roles.length > 0
  return workflow
}

function normalizeWorkflow(input: Playbook): PlaybookWorkflow {
  const parsed = {
    plan: normalizeStage(input.workflow?.plan),
    build: normalizeStage(input.workflow?.build),
    test: normalizeStage(input.workflow?.test),
  }

  const hasRoles = parsed.plan.roles.length > 0 || parsed.build.roles.length > 0 || parsed.test.roles.length > 0
  if (!hasRoles) {
    return fallbackWorkflowFromPhases(Array.isArray(input.phases) ? input.phases : [])
  }
  return parsed
}

function normalizePlaybook(input: Playbook): Playbook {
  return {
    ...input,
    phases: Array.isArray(input.phases) ? input.phases : DEFAULT_PLAYBOOK.phases,
    workflow: normalizeWorkflow(input),
  }
}

function cloneWorkflow(workflow: PlaybookWorkflow): PlaybookWorkflow {
  return {
    plan: {
      enabled: workflow.plan.enabled,
      roles: workflow.plan.roles.map((role) => ({
        ...role,
        allowed_agents: [...role.allowed_agents],
        inputs_required: [...(role.inputs_required ?? [])],
        actions_allowed: [...(role.actions_allowed ?? [])],
        handoff_to: [...(role.handoff_to ?? [])],
        completion_criteria: [...(role.completion_criteria ?? [])],
        outputs_contract: {
          type: role.outputs_contract?.type ?? '',
          required: [...(role.outputs_contract?.required ?? [])],
        },
        gates: {
          requires_user_approval: !!role.gates?.requires_user_approval,
          pass_condition: role.gates?.pass_condition ?? '',
        },
        retry_policy: {
          max_iterations: role.retry_policy?.max_iterations ?? 0,
          escalate_on: [...(role.retry_policy?.escalate_on ?? [])],
        },
      })),
      stage_policy: { ...(workflow.plan.stage_policy ?? {}) },
    },
    build: {
      enabled: workflow.build.enabled,
      roles: workflow.build.roles.map((role) => ({
        ...role,
        allowed_agents: [...role.allowed_agents],
        inputs_required: [...(role.inputs_required ?? [])],
        actions_allowed: [...(role.actions_allowed ?? [])],
        handoff_to: [...(role.handoff_to ?? [])],
        completion_criteria: [...(role.completion_criteria ?? [])],
        outputs_contract: {
          type: role.outputs_contract?.type ?? '',
          required: [...(role.outputs_contract?.required ?? [])],
        },
        gates: {
          requires_user_approval: !!role.gates?.requires_user_approval,
          pass_condition: role.gates?.pass_condition ?? '',
        },
        retry_policy: {
          max_iterations: role.retry_policy?.max_iterations ?? 0,
          escalate_on: [...(role.retry_policy?.escalate_on ?? [])],
        },
      })),
      stage_policy: { ...(workflow.build.stage_policy ?? {}) },
    },
    test: {
      enabled: workflow.test.enabled,
      roles: workflow.test.roles.map((role) => ({
        ...role,
        allowed_agents: [...role.allowed_agents],
        inputs_required: [...(role.inputs_required ?? [])],
        actions_allowed: [...(role.actions_allowed ?? [])],
        handoff_to: [...(role.handoff_to ?? [])],
        completion_criteria: [...(role.completion_criteria ?? [])],
        outputs_contract: {
          type: role.outputs_contract?.type ?? '',
          required: [...(role.outputs_contract?.required ?? [])],
        },
        gates: {
          requires_user_approval: !!role.gates?.requires_user_approval,
          pass_condition: role.gates?.pass_condition ?? '',
        },
        retry_policy: {
          max_iterations: role.retry_policy?.max_iterations ?? 0,
          escalate_on: [...(role.retry_policy?.escalate_on ?? [])],
        },
      })),
      stage_policy: { ...(workflow.test.stage_policy ?? {}) },
    },
  }
}

const STAGE_META: Record<WorkflowStageKey, { label: string; icon: typeof Map }> = {
  plan: { label: 'Plan Stage', icon: Map },
  build: { label: 'Build Stage', icon: Hammer },
  test: { label: 'Test Stage', icon: ShieldCheck },
}

function InfoHint({ text }: { text: string }) {
  return (
    <span className="settings-info-icon" title={text} aria-label={text}>
      <CircleHelp size={13} />
    </span>
  )
}

export default function Settings() {
  const [activeTab, setActiveTab] = useState<TabKey>('agents')
  const [agents, setAgents] = useState<AgentConfig[]>([])
  const [playbooks, setPlaybooks] = useState<Playbook[]>([])
  const [selectedAgentID, setSelectedAgentID] = useState<string>('')
  const [selectedPlaybookID, setSelectedPlaybookID] = useState<string>('')
  const [agentDraft, setAgentDraft] = useState<AgentConfig>(DEFAULT_AGENT)
  const [playbookDraft, setPlaybookDraft] = useState<Playbook>(DEFAULT_PLAYBOOK)
  const [loading, setLoading] = useState<boolean>(true)
  const [busy, setBusy] = useState<boolean>(false)
  const [message, setMessage] = useState<string>('')
  const [asrSettings, setAsrSettings] = useState(() => loadASRSettings())
  const [asrSaved, setAsrSaved] = useState(false)
  const [deleteTarget, setDeleteTarget] = useState<{ kind: 'agent' | 'playbook'; id: string; name: string } | null>(null)

  const isNewAgent = selectedAgentID === ''
  const isNewPlaybook = selectedPlaybookID === ''

  useEffect(() => {
    let cancelled = false
    async function load() {
      setLoading(true)
      try {
        const [agentsData, playbooksData] = await Promise.all([listAgents<AgentConfig[]>(), listPlaybooks<Playbook[]>()])
        if (cancelled) {
          return
        }
        const normalizedPlaybooks = playbooksData.map(normalizePlaybook)
        setAgents(agentsData)
        setPlaybooks(normalizedPlaybooks)
        if (agentsData.length > 0) {
          setSelectedAgentID(agentsData[0].id)
          setAgentDraft(agentsData[0])
        }
        if (normalizedPlaybooks.length > 0) {
          setSelectedPlaybookID(normalizedPlaybooks[0].id)
          setPlaybookDraft(normalizedPlaybooks[0])
        }
      } catch (error) {
        setMessage(error instanceof Error ? error.message : 'Failed to load settings')
      } finally {
        if (!cancelled) {
          setLoading(false)
        }
      }
    }

    void load()
    return () => {
      cancelled = true
    }
  }, [])

  const selectedAgent = useMemo(() => agents.find((item) => item.id === selectedAgentID) ?? null, [agents, selectedAgentID])
  const selectedPlaybook = useMemo(() => playbooks.find((item) => item.id === selectedPlaybookID) ?? null, [playbooks, selectedPlaybookID])

  function startNewAgent() {
    setSelectedAgentID('')
    setAgentDraft(DEFAULT_AGENT)
    setMessage('')
  }

  function cancelNewAgent() {
    if (agents.length > 0) {
      setSelectedAgentID(agents[0].id)
      setAgentDraft(agents[0])
    } else {
      setSelectedAgentID('')
      setAgentDraft(DEFAULT_AGENT)
    }
    setMessage('')
  }

  function startNewPlaybook() {
    setSelectedPlaybookID('')
    setPlaybookDraft(DEFAULT_PLAYBOOK)
    setMessage('')
  }

  function selectAgent(id: string) {
    const found = agents.find((item) => item.id === id)
    if (!found) {
      return
    }
    setSelectedAgentID(id)
    setAgentDraft(found)
    setMessage('')
  }

  function selectPlaybook(id: string) {
    const found = playbooks.find((item) => item.id === id)
    if (!found) {
      return
    }
    setSelectedPlaybookID(id)
    setPlaybookDraft(found)
    setMessage('')
  }

  function updateWorkflow(stageKey: WorkflowStageKey, updater: (stage: PlaybookWorkflowStage) => PlaybookWorkflowStage) {
    setPlaybookDraft((prev) => {
      const nextWorkflow = cloneWorkflow(prev.workflow)
      nextWorkflow[stageKey] = updater(nextWorkflow[stageKey])
      return { ...prev, workflow: nextWorkflow }
    })
  }

  function toggleStage(stageKey: WorkflowStageKey, enabled: boolean) {
    updateWorkflow(stageKey, (stage) => ({
      ...stage,
      enabled,
      roles:
        enabled && stage.roles.length === 0
          ? [
              createDefaultRole(
                stageKey === 'build' ? 'implementer' : stageKey,
                stageKey === 'plan' ? 'planner' : stageKey === 'test' ? 'tester' : 'worker',
              ),
            ]
          : stage.roles,
    }))
  }

  function addRole(stageKey: WorkflowStageKey) {
    updateWorkflow(stageKey, (stage) => ({
      ...stage,
      roles: [
        ...stage.roles,
        createDefaultRole('', stageKey === 'plan' ? 'planner' : stageKey === 'test' ? 'tester' : 'worker'),
      ],
    }))
  }

  function removeRole(stageKey: WorkflowStageKey, index: number) {
    updateWorkflow(stageKey, (stage) => ({ ...stage, roles: stage.roles.filter((_, i) => i !== index) }))
  }

  function updateRole(stageKey: WorkflowStageKey, index: number, patch: Partial<PlaybookWorkflowRole>) {
    updateWorkflow(stageKey, (stage) => ({
      ...stage,
      roles: stage.roles.map((role, i) => (i === index ? { ...role, ...patch } : role)),
    }))
  }

  function updateStagePolicy(stageKey: WorkflowStageKey, patch: NonNullable<PlaybookWorkflowStage['stage_policy']>) {
    updateWorkflow(stageKey, (stage) => ({
      ...stage,
      stage_policy: { ...(stage.stage_policy ?? {}), ...patch },
    }))
  }

  function applyRoleTemplate(stageKey: WorkflowStageKey, roleIndex: number, templateKey: RoleTemplateKey) {
    const template = ROLE_TEMPLATES[templateKey]
    if (!template) {
      return
    }
    updateRole(stageKey, roleIndex, {
      ...template,
      outputs_contract: {
        type: template.outputs_contract?.type ?? '',
        required: [...(template.outputs_contract?.required ?? [])],
      },
      gates: {
        requires_user_approval: !!template.gates?.requires_user_approval,
        pass_condition: template.gates?.pass_condition ?? '',
      },
      retry_policy: {
        max_iterations: template.retry_policy?.max_iterations ?? 0,
        escalate_on: [...(template.retry_policy?.escalate_on ?? [])],
      },
      inputs_required: [...(template.inputs_required ?? [])],
      actions_allowed: [...(template.actions_allowed ?? [])],
      handoff_to: [...(template.handoff_to ?? [])],
      completion_criteria: [...(template.completion_criteria ?? [])],
    })
  }

  function toggleRoleAgent(stageKey: WorkflowStageKey, roleIndex: number, agentID: string, checked: boolean) {
    const current = playbookDraft.workflow[stageKey].roles[roleIndex]
    if (!current) {
      return
    }
    const allowed = checked
      ? [...current.allowed_agents.filter((item) => item !== agentID), agentID]
      : current.allowed_agents.filter((item) => item !== agentID)
    updateRole(stageKey, roleIndex, { allowed_agents: allowed })
  }

  function toggleRoleAction(stageKey: WorkflowStageKey, roleIndex: number, action: string, checked: boolean) {
    const current = playbookDraft.workflow[stageKey].roles[roleIndex]
    if (!current) {
      return
    }
    const next = checked
      ? [...(current.actions_allowed ?? []).filter((item) => item !== action), action]
      : (current.actions_allowed ?? []).filter((item) => item !== action)
    updateRole(stageKey, roleIndex, { actions_allowed: next })
  }

  async function saveAgent() {
    setBusy(true)
    setMessage('')
    try {
      if (isNewAgent) {
        const created = await createAgent<AgentConfig>(agentDraft)
        setAgents((prev) => [...prev.filter((item) => item.id !== created.id), created].sort((a, b) => a.name.localeCompare(b.name)))
        setSelectedAgentID(created.id)
        setAgentDraft(created)
      } else {
        const updated = await updateAgent<AgentConfig>(selectedAgentID, agentDraft)
        setAgents((prev) => prev.map((item) => (item.id === updated.id ? updated : item)))
        setAgentDraft(updated)
      }
      setMessage('Agent saved.')
    } catch (error) {
      setMessage(error instanceof Error ? error.message : 'Failed to save agent')
    } finally {
      setBusy(false)
    }
  }

  async function removeAgent() {
    if (!selectedAgentID || !selectedAgent) {
      return
    }

    setBusy(true)
    setMessage('')
    try {
      await deleteAgent(selectedAgentID)
      const next = agents.filter((item) => item.id !== selectedAgentID)
      setAgents(next)
      if (next.length > 0) {
        setSelectedAgentID(next[0].id)
        setAgentDraft(next[0])
      } else {
        startNewAgent()
      }
      setMessage('Agent deleted.')
    } catch (error) {
      setMessage(error instanceof Error ? error.message : 'Failed to delete agent')
    } finally {
      setBusy(false)
    }
  }

  async function savePlaybook() {
    const validationError = validatePlaybookDraft(playbookDraft, isNewPlaybook)
    if (validationError) {
      setMessage(validationError)
      return
    }
    setBusy(true)
    setMessage('')
    try {
      const payload: Playbook = {
        ...playbookDraft,
      }
      let savedID = selectedPlaybookID
      if (isNewPlaybook) {
        const created = normalizePlaybook(await createPlaybook<Playbook>(payload))
        savedID = created.id
      } else {
        const updated = normalizePlaybook(await updatePlaybook<Playbook>(selectedPlaybookID, payload))
        savedID = updated.id
      }
      const latest = (await listPlaybooks<Playbook[]>()).map(normalizePlaybook)
      setPlaybooks(latest)
      const persisted = latest.find((item) => item.id === savedID)
      if (persisted) {
        setSelectedPlaybookID(persisted.id)
        setPlaybookDraft(persisted)
        setMessage(`Playbook saved: ${persisted.name}`)
      } else {
        setMessage('Playbook save completed but refresh failed. Please reload playbooks.')
      }
    } catch (error) {
      setMessage(parseAPIErrorMessage(error))
    } finally {
      setBusy(false)
    }
  }

  async function removePlaybook() {
    if (!selectedPlaybookID || !selectedPlaybook) {
      return
    }

    setBusy(true)
    setMessage('')
    try {
      await deletePlaybook(selectedPlaybookID)
      const next = playbooks.filter((item) => item.id !== selectedPlaybookID)
      setPlaybooks(next)
      if (next.length > 0) {
        setSelectedPlaybookID(next[0].id)
        setPlaybookDraft(next[0])
      } else {
        startNewPlaybook()
      }
      setMessage('Playbook deleted.')
    } catch (error) {
      setMessage(error instanceof Error ? error.message : 'Failed to delete playbook')
    } finally {
      setBusy(false)
    }
  }

  function saveASR() {
    saveASRSettings({
      appID: asrSettings.appID.trim(),
      accessKey: asrSettings.accessKey.trim(),
    })
    setAsrSaved(true)
    window.setTimeout(() => setAsrSaved(false), 1500)
  }

  function requestRemoveAgent() {
    if (!selectedAgentID || !selectedAgent) {
      return
    }
    setDeleteTarget({ kind: 'agent', id: selectedAgentID, name: selectedAgent.name })
  }

  function requestRemovePlaybook() {
    if (!selectedPlaybookID || !selectedPlaybook) {
      return
    }
    setDeleteTarget({ kind: 'playbook', id: selectedPlaybookID, name: selectedPlaybook.name })
  }

  async function confirmDelete() {
    if (!deleteTarget) {
      return
    }
    const pending = deleteTarget
    setDeleteTarget(null)

    if (pending.kind === 'agent' && pending.id === selectedAgentID) {
      await removeAgent()
      return
    }
    if (pending.kind === 'playbook' && pending.id === selectedPlaybookID) {
      await removePlaybook()
    }
  }

  if (loading) {
    return (
      <section className="page-block settings-page">
        <h2>Settings</h2>
        <p>Loading configuration...</p>
      </section>
    )
  }

  return (
    <section className="page-block settings-page">
      <h2>Settings</h2>
      <div className="settings-tabs">
        <button type="button" className={`secondary-btn ${activeTab === 'agents' ? 'active' : ''}`} onClick={() => setActiveTab('agents')}>
          Agent Registry
        </button>
        <button type="button" className={`secondary-btn ${activeTab === 'playbooks' ? 'active' : ''}`} onClick={() => setActiveTab('playbooks')}>
          Playbooks
        </button>
        <button type="button" className={`secondary-btn ${activeTab === 'asr' ? 'active' : ''}`} onClick={() => setActiveTab('asr')}>
          ASR
        </button>
      </div>

      {message ? <p className="settings-message">{message}</p> : null}

      {activeTab === 'agents' ? (
        <div className="settings-grid">
          <aside className="settings-list">
            <button type="button" className="primary-btn" onClick={startNewAgent}>
              + New Agent
            </button>
            {isNewAgent && (
              <button type="button" className="session-row active">
                <strong>{agentDraft.name.trim() || 'New Agent (Draft)'}</strong>
                <small>{agentDraft.id.trim() || 'unsaved'}</small>
              </button>
            )}
            {agents.map((item) => (
              <button
                key={item.id}
                type="button"
                className={`session-row ${item.id === selectedAgentID ? 'active' : ''}`}
                onClick={() => selectAgent(item.id)}
              >
                <strong>{item.name}</strong>
                <small>{item.id}</small>
              </button>
            ))}
          </aside>

          <div className="settings-editor">
            <label>
              ID
              <input
                value={agentDraft.id}
                disabled={!isNewAgent}
                onChange={(event) => setAgentDraft((prev) => ({ ...prev, id: event.target.value.trim().toLowerCase() }))}
              />
            </label>
            <label>
              Name
              <input value={agentDraft.name} onChange={(event) => setAgentDraft((prev) => ({ ...prev, name: event.target.value }))} />
            </label>
            <label>
              Command
              <textarea
                rows={3}
                value={agentDraft.command}
                placeholder={`claude --dangerously-skip-permissions\n# or multiline bootstrap commands`}
                onChange={(event) => setAgentDraft((prev) => ({ ...prev, command: event.target.value }))}
              />
            </label>
            <label>
              Session Model (optional)
              <input
                value={agentDraft.model ?? ''}
                onChange={(event) => setAgentDraft((prev) => ({ ...prev, model: event.target.value }))}
                placeholder="Used for normal session launches"
              />
            </label>
            <label>
              Max Parallel Agents
              <input
                min={1}
                max={64}
                type="number"
                value={agentDraft.max_parallel_agents ?? 1}
                onChange={(event) =>
                  setAgentDraft((prev) => ({
                    ...prev,
                    max_parallel_agents: clampParallelAgents(Number(event.target.value || 1)),
                  }))
                }
              />
            </label>
            <label className="settings-field-checkbox">
              <span>Can Act As Orchestrator</span>
              <input
                checked={!!agentDraft.supports_orchestrator}
                onChange={(event) =>
                  setAgentDraft((prev) => ({
                    ...prev,
                    supports_orchestrator: event.target.checked,
                    orchestrator_provider: prev.orchestrator_provider || 'anthropic',
                  }))
                }
                type="checkbox"
              />
            </label>
            {agentDraft.supports_orchestrator && (
              <>
                <label>
                  Orchestrator Model
                  <input
                    value={agentDraft.model ?? ''}
                    onChange={(event) => setAgentDraft((prev) => ({ ...prev, model: event.target.value }))}
                    placeholder="e.g. claude-sonnet-4-5 / gpt-5-codex"
                  />
                </label>
                <label>
                  Orchestrator Format
                  <select
                    value={agentDraft.orchestrator_provider ?? 'anthropic'}
                    onChange={(event) =>
                      setAgentDraft((prev) => ({
                        ...prev,
                        orchestrator_provider: event.target.value as 'anthropic' | 'openai',
                      }))
                    }
                  >
                    <option value="anthropic">anthropic</option>
                    <option value="openai">openai</option>
                  </select>
                </label>
                <label>
                  Orchestrator API Key
                  <input
                    type="password"
                    value={agentDraft.orchestrator_api_key ?? ''}
                    onChange={(event) => setAgentDraft((prev) => ({ ...prev, orchestrator_api_key: event.target.value }))}
                    placeholder="sk-..."
                  />
                </label>
                <label>
                  Orchestrator API Endpoint
                  <input
                    value={agentDraft.orchestrator_api_base ?? ''}
                    onChange={(event) => setAgentDraft((prev) => ({ ...prev, orchestrator_api_base: event.target.value }))}
                    placeholder="https://api.anthropic.com/v1/messages"
                  />
                </label>
              </>
            )}
            <label>
              Agent Bio
              <textarea
                rows={4}
                value={agentDraft.notes ?? ''}
                onChange={(event) => setAgentDraft((prev) => ({ ...prev, notes: event.target.value }))}
                placeholder="Describe this agent like an employee profile: strengths, stack, preferred tasks, constraints."
              />
            </label>
            <div className="settings-actions">
              <button type="button" className="primary-btn" disabled={busy} onClick={() => void saveAgent()}>
                Save Agent
              </button>
              {isNewAgent ? (
                <button type="button" className="secondary-btn" disabled={busy} onClick={cancelNewAgent}>
                  Cancel
                </button>
              ) : null}
              {!isNewAgent ? (
                <button type="button" className="action-btn danger" disabled={busy} onClick={requestRemoveAgent}>
                  Delete Agent
                </button>
              ) : null}
            </div>
          </div>
        </div>
      ) : activeTab === 'playbooks' ? (
        <div className="settings-grid">
          <aside className="settings-list">
            <button type="button" className="primary-btn" onClick={startNewPlaybook}>
              + New Playbook
            </button>
            {isNewPlaybook && (
              <button type="button" className="session-row active">
                <strong>{playbookDraft.name.trim() || 'New Playbook (Draft)'}</strong>
                <small>{playbookDraft.id.trim() || 'unsaved'}</small>
              </button>
            )}
            {playbooks.map((item) => (
              <button
                key={item.id}
                type="button"
                className={`session-row ${item.id === selectedPlaybookID ? 'active' : ''}`}
                onClick={() => selectPlaybook(item.id)}
              >
                <strong>{item.name}</strong>
                <small>{item.id}</small>
              </button>
            ))}
          </aside>

          <div className="settings-editor">
            <label>
              ID
              <input
                value={playbookDraft.id}
                disabled={!isNewPlaybook}
                onChange={(event) => setPlaybookDraft((prev) => ({ ...prev, id: event.target.value.trim().toLowerCase() }))}
              />
            </label>
            <label>
              Name
              <input value={playbookDraft.name} onChange={(event) => setPlaybookDraft((prev) => ({ ...prev, name: event.target.value }))} />
            </label>
            <label>
              Description
              <input value={playbookDraft.description} onChange={(event) => setPlaybookDraft((prev) => ({ ...prev, description: event.target.value }))} />
            </label>

            <div className="settings-workflow-editor">
              {(Object.keys(STAGE_META) as WorkflowStageKey[]).map((stageKey) => {
                const stage = playbookDraft.workflow[stageKey]
                const stageMeta = STAGE_META[stageKey]
                const StageIcon = stageMeta.icon
                return (
                  <section className={`settings-stage-card ${stage.enabled ? 'enabled' : 'disabled'}`} key={stageKey}>
                    <header className="settings-stage-header">
                      <h4>
                        <StageIcon size={16} />
                        {stageMeta.label}
                      </h4>
                      <label className="settings-field-checkbox">
                        <span>Enabled</span>
                        <input checked={stage.enabled} onChange={(event) => toggleStage(stageKey, event.target.checked)} type="checkbox" />
                      </label>
                    </header>

                    {!stage.enabled ? (
                      <p className="settings-stage-hint">This stage is disabled for this playbook.</p>
                    ) : (
                      <>
                        {stage.roles.map((role, roleIndex) => (
                          <div className="settings-stage-role" key={`${stageKey}-${roleIndex}`}>
                            <div className="settings-stage-role-head">
                              <strong>Role {roleIndex + 1}</strong>
                              <button type="button" className="action-btn danger" onClick={() => removeRole(stageKey, roleIndex)}>
                                <Trash2 size={14} />
                                Remove
                              </button>
                            </div>

                            <div className="settings-actions">
                              <button type="button" className="secondary-btn" onClick={() => applyRoleTemplate(stageKey, roleIndex, 'planner')}>
                                Planner Template
                              </button>
                              <button type="button" className="secondary-btn" onClick={() => applyRoleTemplate(stageKey, roleIndex, 'worker')}>
                                Worker Template
                              </button>
                              <button type="button" className="secondary-btn" onClick={() => applyRoleTemplate(stageKey, roleIndex, 'reviewer')}>
                                Reviewer Template
                              </button>
                              <button type="button" className="secondary-btn" onClick={() => applyRoleTemplate(stageKey, roleIndex, 'tester')}>
                                Tester Template
                              </button>
                            </div>

                            <label>
                              <span className="settings-label-inline">Role Name</span>
                              <input value={role.name} onChange={(event) => updateRole(stageKey, roleIndex, { name: event.target.value })} />
                            </label>
                            <label>
                              <span className="settings-label-inline">
                                Role Type
                                <InfoHint text="Choose planner, worker, reviewer, or tester. Changing this preloads recommended detailed defaults." />
                              </span>
                              <select
                                value={role.mode ?? 'worker'}
                                onChange={(event) => {
                                  const nextMode = event.target.value as PlaybookWorkflowRole['mode']
                                  updateRole(stageKey, roleIndex, withDetailedDefaults({ ...role, mode: nextMode }))
                                }}
                              >
                                <option value="planner">planner</option>
                                <option value="worker">worker</option>
                                <option value="reviewer">reviewer</option>
                                <option value="tester">tester</option>
                              </select>
                            </label>

                            <div className="settings-role-agents">
                              <span className="settings-label-inline">
                                Available Models (Agent IDs)
                                <InfoHint text="Basic setting: select which agents this role can use." />
                              </span>
                              {agents.length === 0 ? (
                                <small>No agents available. Create agents first.</small>
                              ) : (
                                <div className="settings-role-agents-grid">
                                  {agents.map((agent) => (
                                    <label key={`${stageKey}-${roleIndex}-${agent.id}`} className="settings-role-agent-item">
                                      <input
                                        checked={role.allowed_agents.includes(agent.id)}
                                        onChange={(event) => toggleRoleAgent(stageKey, roleIndex, agent.id, event.target.checked)}
                                        type="checkbox"
                                      />
                                      <span>{agent.id}</span>
                                    </label>
                                  ))}
                                </div>
                              )}
                            </div>

                            <label>
                              <span className="settings-label-inline">
                                Suggested Prompt (optional)
                                <InfoHint text="Basic setting: orchestrator forwards this guidance when dispatching this role." />
                              </span>
                              <textarea
                                rows={2}
                                value={role.suggested_prompt ?? ''}
                                onChange={(event) => updateRole(stageKey, roleIndex, { suggested_prompt: event.target.value })}
                              />
                            </label>

                            <details className="settings-role-details">
                              <summary>
                                <span>Detailed Settings</span>
                                <InfoHint text="Advanced role controls. Defaults are prefilled so you only edit what you need." />
                              </summary>

                              <label>
                                <span className="settings-label-inline">
                                  Responsibilities
                                  <InfoHint text="What this role is accountable for in plain language." />
                                </span>
                                <textarea
                                  rows={2}
                                  value={role.responsibilities}
                                  onChange={(event) => updateRole(stageKey, roleIndex, { responsibilities: event.target.value })}
                                />
                              </label>

                              <label>
                                <span className="settings-label-inline">
                                  Inputs Required (comma-separated)
                                  <InfoHint text="Artifacts or context this role must have before starting." />
                                </span>
                                <input
                                  value={(role.inputs_required ?? []).join(', ')}
                                  onChange={(event) => updateRole(stageKey, roleIndex, { inputs_required: parseCSV(event.target.value) })}
                                />
                              </label>

                              <div className="settings-role-agents">
                                <span className="settings-label-inline">
                                  Actions Allowed
                                  <InfoHint text="Tool allowlist for this role. Keep minimal for safety." />
                                </span>
                                <div className="settings-role-agents-grid">
                                  {TOOL_OPTIONS.map((action) => (
                                    <label key={`${stageKey}-${roleIndex}-action-${action}`} className="settings-role-agent-item">
                                      <input
                                        checked={(role.actions_allowed ?? []).includes(action)}
                                        onChange={(event) => toggleRoleAction(stageKey, roleIndex, action, event.target.checked)}
                                        type="checkbox"
                                      />
                                      <span>{action}</span>
                                    </label>
                                  ))}
                                </div>
                              </div>

                              <label>
                                <span className="settings-label-inline">
                                  Handoff To (comma-separated role names)
                                  <InfoHint text="Roles that should receive control after this role finishes." />
                                </span>
                                <input
                                  value={(role.handoff_to ?? []).join(', ')}
                                  onChange={(event) => updateRole(stageKey, roleIndex, { handoff_to: parseCSV(event.target.value) })}
                                />
                              </label>

                              <label>
                                <span className="settings-label-inline">
                                  Completion Criteria (comma-separated)
                                  <InfoHint text="Conditions used to mark this role as complete." />
                                </span>
                                <input
                                  value={(role.completion_criteria ?? []).join(', ')}
                                  onChange={(event) => updateRole(stageKey, roleIndex, { completion_criteria: parseCSV(event.target.value) })}
                                />
                              </label>

                              <label>
                                <span className="settings-label-inline">
                                  Outputs Contract Type
                                  <InfoHint text="Structured output class expected from this role (e.g. review_result)." />
                                </span>
                                <input
                                  value={role.outputs_contract?.type ?? ''}
                                  onChange={(event) =>
                                    updateRole(stageKey, roleIndex, {
                                      outputs_contract: {
                                        type: event.target.value,
                                        required: [...(role.outputs_contract?.required ?? [])],
                                      },
                                    })
                                  }
                                />
                              </label>

                              <label>
                                <span className="settings-label-inline">
                                  Outputs Contract Required Fields (comma-separated)
                                  <InfoHint text="Required keys in role output payload." />
                                </span>
                                <input
                                  value={(role.outputs_contract?.required ?? []).join(', ')}
                                  onChange={(event) =>
                                    updateRole(stageKey, roleIndex, {
                                      outputs_contract: {
                                        type: role.outputs_contract?.type ?? '',
                                        required: parseCSV(event.target.value),
                                      },
                                    })
                                  }
                                />
                              </label>

                              <label className="settings-field-checkbox">
                                <span className="settings-label-inline">
                                  Gate: Requires User Approval
                                  <InfoHint text="If enabled, this role cannot run mutating actions without explicit user confirmation." />
                                </span>
                                <input
                                  type="checkbox"
                                  checked={!!role.gates?.requires_user_approval}
                                  onChange={(event) =>
                                    updateRole(stageKey, roleIndex, {
                                      gates: {
                                        requires_user_approval: event.target.checked,
                                        pass_condition: role.gates?.pass_condition ?? '',
                                      },
                                    })
                                  }
                                />
                              </label>

                              <label>
                                <span className="settings-label-inline">
                                  Gate: Pass Condition
                                  <InfoHint text="Expression or marker that indicates the role completed successfully." />
                                </span>
                                <input
                                  value={role.gates?.pass_condition ?? ''}
                                  onChange={(event) =>
                                    updateRole(stageKey, roleIndex, {
                                      gates: {
                                        requires_user_approval: !!role.gates?.requires_user_approval,
                                        pass_condition: event.target.value,
                                      },
                                    })
                                  }
                                />
                              </label>

                              <label>
                                <span className="settings-label-inline">
                                  Retry Policy: Max Iterations
                                  <InfoHint text="Maximum automatic loops before escalating." />
                                </span>
                                <input
                                  type="number"
                                  min={0}
                                  value={role.retry_policy?.max_iterations ?? 0}
                                  onChange={(event) =>
                                    updateRole(stageKey, roleIndex, {
                                      retry_policy: {
                                        max_iterations: clampNonNegative(Number(event.target.value || 0)),
                                        escalate_on: [...(role.retry_policy?.escalate_on ?? [])],
                                      },
                                    })
                                  }
                                />
                              </label>

                              <label>
                                <span className="settings-label-inline">
                                  Retry Policy: Escalate On (comma-separated)
                                  <InfoHint text="Signals that should trigger human escalation." />
                                </span>
                                <input
                                  value={(role.retry_policy?.escalate_on ?? []).join(', ')}
                                  onChange={(event) =>
                                    updateRole(stageKey, roleIndex, {
                                      retry_policy: {
                                        max_iterations: role.retry_policy?.max_iterations ?? 0,
                                        escalate_on: parseCSV(event.target.value),
                                      },
                                    })
                                  }
                                />
                              </label>
                            </details>
                          </div>
                        ))}

                        <div className="settings-stage-role">
                          <div className="settings-stage-role-head">
                            <strong>Stage Policy</strong>
                          </div>
                          <details className="settings-role-details">
                            <summary>
                              <span>Detailed Stage Policy</span>
                              <InfoHint text="Advanced stage-level gates and parallelism limits." />
                            </summary>
                            <label>
                              <span className="settings-label-inline">
                                Enter Gate
                                <InfoHint text="Condition that must hold before this stage can start." />
                              </span>
                              <input
                                value={stage.stage_policy?.enter_gate ?? ''}
                                onChange={(event) => updateStagePolicy(stageKey, { enter_gate: event.target.value })}
                              />
                            </label>
                            <label>
                              <span className="settings-label-inline">
                                Exit Gate
                                <InfoHint text="Condition that must hold before transitioning to the next stage." />
                              </span>
                              <input
                                value={stage.stage_policy?.exit_gate ?? ''}
                                onChange={(event) => updateStagePolicy(stageKey, { exit_gate: event.target.value })}
                              />
                            </label>
                            <label>
                              <span className="settings-label-inline">
                                Max Parallel Worktrees
                                <InfoHint text="Upper bound for concurrent worktrees in this stage. 0 means default behavior." />
                              </span>
                              <input
                                type="number"
                                min={0}
                                value={stage.stage_policy?.max_parallel_worktrees ?? 0}
                                onChange={(event) =>
                                  updateStagePolicy(stageKey, { max_parallel_worktrees: clampNonNegative(Number(event.target.value || 0)) })
                                }
                              />
                            </label>
                          </details>
                        </div>

                        <button type="button" className="secondary-btn settings-stage-add" onClick={() => addRole(stageKey)}>
                          <Plus size={14} />
                          Add Role
                        </button>
                      </>
                    )}
                  </section>
                )
              })}
            </div>
            <div className="settings-actions">
              <button type="button" className="primary-btn" disabled={busy} onClick={() => void savePlaybook()}>
                Save Playbook
              </button>
              {!isNewPlaybook ? (
                <button type="button" className="action-btn danger" disabled={busy} onClick={requestRemovePlaybook}>
                  Delete Playbook
                </button>
              ) : null}
            </div>
          </div>
        </div>
      ) : (
        <div className="settings-card">
          <h3>Volcengine ASR</h3>
          <p>Configure speech-to-text credentials for PM Chat microphone input.</p>

          <label className="settings-field" htmlFor="asr-app-id">
            <span>App ID</span>
            <input
              id="asr-app-id"
              value={asrSettings.appID}
              onChange={(event) => setAsrSettings((prev) => ({ ...prev, appID: event.target.value }))}
              placeholder="volc app id"
            />
          </label>

          <label className="settings-field" htmlFor="asr-access-key">
            <span>Access Key</span>
            <input
              id="asr-access-key"
              type="password"
              value={asrSettings.accessKey}
              onChange={(event) => setAsrSettings((prev) => ({ ...prev, accessKey: event.target.value }))}
              placeholder="volc access key"
            />
          </label>

          <div className="settings-actions">
            <button className="primary-btn" type="button" onClick={saveASR}>
              Save
            </button>
            {asrSaved && <small>Saved</small>}
          </div>
        </div>
      )}

      <Modal onClose={() => setDeleteTarget(null)} open={!!deleteTarget} title="Confirm Delete">
        <p>Delete {deleteTarget?.kind} &quot;{deleteTarget?.name}&quot;? This cannot be undone.</p>
        <div className="settings-actions">
          <button className="secondary-btn" onClick={() => setDeleteTarget(null)} type="button">
            Cancel
          </button>
          <button className="action-btn danger" onClick={() => void confirmDelete()} type="button">
            Delete
          </button>
        </div>
      </Modal>
    </section>
  )
}
