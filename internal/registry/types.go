package registry

type AgentConfig struct {
	ID                    string   `yaml:"id" json:"id"`
	Name                  string   `yaml:"name" json:"name"`
	Command               string   `yaml:"command" json:"command"`
	ResumeCommand         string   `yaml:"resume_command,omitempty" json:"resume_command,omitempty"`
	HeadlessCommand       string   `yaml:"headless_command,omitempty" json:"headless_command,omitempty"`
	Capabilities          []string `yaml:"capabilities" json:"capabilities"`
	Languages             []string `yaml:"languages" json:"languages"`
	CostTier              string   `yaml:"cost_tier" json:"cost_tier"`
	SpeedTier             string   `yaml:"speed_tier" json:"speed_tier"`
	SupportsSessionResume bool     `yaml:"supports_session_resume" json:"supports_session_resume"`
	SupportsHeadless      bool     `yaml:"supports_headless" json:"supports_headless"`
	AutoAcceptMode        string   `yaml:"auto_accept_mode,omitempty" json:"auto_accept_mode,omitempty"`
}
