package playbook

type Playbook struct {
	ID                  string  `yaml:"id,omitempty" json:"id"`
	Name                string  `yaml:"name" json:"name"`
	Description         string  `yaml:"description" json:"description"`
	Match               Match   `yaml:"match" json:"match"`
	Phases              []Phase `yaml:"phases" json:"phases"`
	ParallelismStrategy string  `yaml:"parallelism_strategy" json:"parallelism_strategy"`
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
