package playbook

type Playbook struct {
	ID                  string   `yaml:"id,omitempty" json:"id"`
	Name                string   `yaml:"name" json:"name"`
	Description         string   `yaml:"description" json:"description"`
	Match               Match    `yaml:"match" json:"match"`
	Phases              []Phase  `yaml:"phases" json:"phases"`
	Workflow            Workflow `yaml:"workflow" json:"workflow"`
	ParallelismStrategy string   `yaml:"parallelism_strategy" json:"parallelism_strategy"`
}

type Match struct {
	Languages       []string `yaml:"languages" json:"languages"`
	ProjectPatterns []string `yaml:"project_patterns" json:"project_patterns"`
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
	Enabled bool        `yaml:"enabled" json:"enabled"`
	Roles   []StageRole `yaml:"roles" json:"roles"`
}

type StageRole struct {
	Name             string   `yaml:"name" json:"name"`
	Responsibilities string   `yaml:"responsibilities" json:"responsibilities"`
	AllowedAgents    []string `yaml:"allowed_agents" json:"allowed_agents"`
	SuggestedPrompt  string   `yaml:"suggested_prompt,omitempty" json:"suggested_prompt,omitempty"`
}
