package configs

import "embed"

// AgentDefaults contains shipped default agent YAML config files.
//
//go:embed agents/*.yaml
var AgentDefaults embed.FS

// PlaybookDefaults contains shipped default playbook YAML config files.
//
//go:embed playbooks/*.yaml
var PlaybookDefaults embed.FS
