package playbook

type Playbook struct {
	ID          string   `yaml:"id,omitempty" json:"id"`
	Name        string   `yaml:"name" json:"name"`
	Description string   `yaml:"description" json:"description"`
	Phases      []Phase  `yaml:"phases" json:"phases"`
	Workflow    Workflow `yaml:"workflow" json:"workflow"`
}

type Phase struct {
	Name        string `yaml:"name" json:"name"`
	Agent       string `yaml:"agent" json:"agent"`
	Role        string `yaml:"role" json:"role"`
	Description string `yaml:"description" json:"description"`
}

type Workflow struct {
	Plan  Stage `yaml:"plan" json:"plan"`
	Build Stage `yaml:"build" json:"build"`
	Test  Stage `yaml:"test" json:"test"`
}

type Stage struct {
	Enabled     bool        `yaml:"enabled" json:"enabled"`
	Roles       []StageRole `yaml:"roles" json:"roles"`
	StagePolicy StagePolicy `yaml:"stage_policy,omitempty" json:"stage_policy,omitempty"`
}

type StageRole struct {
	Name               string          `yaml:"name" json:"name"`
	Responsibilities   string          `yaml:"responsibilities" json:"responsibilities"`
	AllowedAgents      []string        `yaml:"allowed_agents" json:"allowed_agents"`
	SuggestedPrompt    string          `yaml:"suggested_prompt,omitempty" json:"suggested_prompt,omitempty"`
	Mode               string          `yaml:"mode,omitempty" json:"mode,omitempty"`
	InputsRequired     []string        `yaml:"inputs_required,omitempty" json:"inputs_required,omitempty"`
	ActionsAllowed     []string        `yaml:"actions_allowed,omitempty" json:"actions_allowed,omitempty"`
	HandoffTo          []string        `yaml:"handoff_to,omitempty" json:"handoff_to,omitempty"`
	CompletionCriteria []string        `yaml:"completion_criteria,omitempty" json:"completion_criteria,omitempty"`
	OutputsContract    OutputsContract `yaml:"outputs_contract,omitempty" json:"outputs_contract,omitempty"`
	Gates              RoleGates       `yaml:"gates,omitempty" json:"gates,omitempty"`
	RetryPolicy        RetryPolicy     `yaml:"retry_policy,omitempty" json:"retry_policy,omitempty"`
}

type OutputsContract struct {
	Type     string   `yaml:"type,omitempty" json:"type,omitempty"`
	Required []string `yaml:"required,omitempty" json:"required,omitempty"`
}

type RoleGates struct {
	RequiresUserApproval bool   `yaml:"requires_user_approval,omitempty" json:"requires_user_approval,omitempty"`
	PassCondition        string `yaml:"pass_condition,omitempty" json:"pass_condition,omitempty"`
}

type RetryPolicy struct {
	MaxIterations int      `yaml:"max_iterations,omitempty" json:"max_iterations,omitempty"`
	EscalateOn    []string `yaml:"escalate_on,omitempty" json:"escalate_on,omitempty"`
}

type StagePolicy struct {
	EnterGate            string `yaml:"enter_gate,omitempty" json:"enter_gate,omitempty"`
	ExitGate             string `yaml:"exit_gate,omitempty" json:"exit_gate,omitempty"`
	MaxParallelWorktrees int    `yaml:"max_parallel_worktrees,omitempty" json:"max_parallel_worktrees,omitempty"`
}
