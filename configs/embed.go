package configs

import "embed"

// AgentDefaults contains shipped default agent YAML config files.
//
//go:embed agents/*.yaml
var AgentDefaults embed.FS
