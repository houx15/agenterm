package registry

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

var agentIDPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

type Registry struct {
	dir    string
	agents map[string]*AgentConfig
	mu     sync.RWMutex
}

func NewRegistry(dir string) (*Registry, error) {
	if strings.TrimSpace(dir) == "" {
		return nil, errors.New("agents dir is required")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create registry dir: %w", err)
	}
	if err := ensureDefaults(dir); err != nil {
		return nil, err
	}

	r := &Registry{
		dir:    dir,
		agents: make(map[string]*AgentConfig),
	}
	if err := r.Reload(); err != nil {
		return nil, err
	}
	return r, nil
}

func (r *Registry) Get(id string) *AgentConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()

	cfg, ok := r.agents[id]
	if !ok {
		return nil
	}
	return cloneConfig(cfg)
}

func (r *Registry) List() []*AgentConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*AgentConfig, 0, len(r.agents))
	for _, cfg := range r.agents {
		result = append(result, cloneConfig(cfg))
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Name == result[j].Name {
			return result[i].ID < result[j].ID
		}
		return result[i].Name < result[j].Name
	})
	return result
}

func (r *Registry) Reload() error {
	loaded, err := loadDir(r.dir)
	if err != nil {
		return err
	}

	r.mu.Lock()
	r.agents = loaded
	r.mu.Unlock()
	return nil
}

func (r *Registry) Save(cfg *AgentConfig) error {
	if cfg == nil {
		return errors.New("agent config is required")
	}
	clean := cloneConfig(cfg)
	if err := validate(clean); err != nil {
		return err
	}

	data, err := yaml.Marshal(clean)
	if err != nil {
		return fmt.Errorf("marshal agent config: %w", err)
	}
	path := filepath.Join(r.dir, clean.ID+".yaml")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write agent config %q: %w", path, err)
	}

	r.mu.Lock()
	r.agents[clean.ID] = clean
	r.mu.Unlock()
	return nil
}

func (r *Registry) Delete(id string) error {
	if err := validateID(id); err != nil {
		return err
	}
	path := filepath.Join(r.dir, id+".yaml")
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("delete agent config %q: %w", path, err)
	}

	r.mu.Lock()
	delete(r.agents, id)
	r.mu.Unlock()
	return nil
}

func loadDir(dir string) (map[string]*AgentConfig, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read registry dir: %w", err)
	}

	loaded := make(map[string]*AgentConfig)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := strings.ToLower(entry.Name())
		if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		cfg, err := loadFile(path)
		if err != nil {
			return nil, err
		}
		if _, exists := loaded[cfg.ID]; exists {
			return nil, fmt.Errorf("duplicate agent id %q", cfg.ID)
		}
		loaded[cfg.ID] = cfg
	}
	return loaded, nil
}

func loadFile(path string) (*AgentConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read agent config %q: %w", path, err)
	}
	var cfg AgentConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse agent config %q: %w", path, err)
	}
	if err := validate(&cfg); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &cfg, nil
}

func validate(cfg *AgentConfig) error {
	if cfg == nil {
		return errors.New("agent config is required")
	}
	if err := validateID(cfg.ID); err != nil {
		return err
	}
	if strings.TrimSpace(cfg.Name) == "" {
		return errors.New("name is required")
	}
	if strings.TrimSpace(cfg.Command) == "" {
		return errors.New("command is required")
	}
	if cfg.Capabilities == nil {
		cfg.Capabilities = []string{}
	}
	if cfg.Languages == nil {
		cfg.Languages = []string{}
	}
	return nil
}

func validateID(id string) error {
	if strings.TrimSpace(id) == "" {
		return errors.New("id is required")
	}
	if !agentIDPattern.MatchString(id) {
		return errors.New("id must be lowercase alphanumeric with hyphens")
	}
	return nil
}

func cloneConfig(cfg *AgentConfig) *AgentConfig {
	if cfg == nil {
		return nil
	}
	out := *cfg
	out.Capabilities = append([]string(nil), cfg.Capabilities...)
	out.Languages = append([]string(nil), cfg.Languages...)
	return &out
}
