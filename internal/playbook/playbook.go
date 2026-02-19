package playbook

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/user/agenterm/configs"
	"gopkg.in/yaml.v3"
)

var playbookIDPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

var (
	ErrInvalidPlaybook  = errors.New("invalid playbook")
	ErrPlaybookStorage  = errors.New("playbook storage error")
	ErrPlaybookNotFound = errors.New("playbook not found")
)

type Registry struct {
	dir       string
	playbooks map[string]*Playbook
	mu        sync.RWMutex
}

func NewRegistry(dir string) (*Registry, error) {
	if strings.TrimSpace(dir) == "" {
		return nil, errors.New("playbooks dir is required")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create playbooks dir: %w", err)
	}
	if err := ensureDefaults(dir); err != nil {
		return nil, err
	}

	r := &Registry{dir: dir, playbooks: make(map[string]*Playbook)}
	if err := r.Reload(); err != nil {
		return nil, err
	}
	return r, nil
}

func (r *Registry) Get(id string) *Playbook {
	r.mu.RLock()
	defer r.mu.RUnlock()

	pb, ok := r.playbooks[id]
	if !ok {
		return nil
	}
	return clone(pb)
}

func (r *Registry) List() []*Playbook {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*Playbook, 0, len(r.playbooks))
	for _, pb := range r.playbooks {
		result = append(result, clone(pb))
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
	r.playbooks = loaded
	r.mu.Unlock()
	return nil
}

func (r *Registry) Save(pb *Playbook) error {
	if pb == nil {
		return fmt.Errorf("%w: playbook is required", ErrInvalidPlaybook)
	}
	clean := clone(pb)
	if err := normalizeAndValidate(clean); err != nil {
		return err
	}

	data, err := yaml.Marshal(clean)
	if err != nil {
		return fmt.Errorf("%w: marshal playbook: %v", ErrPlaybookStorage, err)
	}
	path := filepath.Join(r.dir, clean.ID+".yaml")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("%w: write playbook %q: %v", ErrPlaybookStorage, path, err)
	}

	r.mu.Lock()
	r.playbooks[clean.ID] = clean
	r.mu.Unlock()
	return nil
}

func (r *Registry) Delete(id string) error {
	if err := validateID(id); err != nil {
		return err
	}

	deleted := false
	for _, ext := range []string{".yaml", ".yml"} {
		path := filepath.Join(r.dir, id+ext)
		err := os.Remove(path)
		if err == nil {
			deleted = true
			continue
		}
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		return fmt.Errorf("%w: delete playbook %q: %v", ErrPlaybookStorage, path, err)
	}
	if !deleted {
		return fmt.Errorf("%w: %s", ErrPlaybookNotFound, id)
	}

	r.mu.Lock()
	delete(r.playbooks, id)
	r.mu.Unlock()
	return nil
}

func (r *Registry) MatchProject(repoPath string) *Playbook {
	_ = repoPath
	r.mu.RLock()
	if len(r.playbooks) == 0 {
		r.mu.RUnlock()
		return nil
	}
	snapshot := make(map[string]*Playbook, len(r.playbooks))
	for id, pb := range r.playbooks {
		snapshot[id] = clone(pb)
	}
	r.mu.RUnlock()

	if fallback, ok := snapshot["pairing-coding"]; ok {
		return clone(fallback)
	}
	ids := make([]string, 0, len(snapshot))
	for id := range snapshot {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	if len(ids) == 0 {
		return nil
	}
	return clone(snapshot[ids[0]])
}

func ensureDefaults(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read playbooks dir: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := strings.ToLower(entry.Name())
		if strings.HasSuffix(name, ".yaml") || strings.HasSuffix(name, ".yml") {
			return nil
		}
	}

	for _, file := range []string{
		"pairing-coding.yaml",
		"tdd.yaml",
		"compound-engineering.yaml",
	} {
		content, err := configs.PlaybookDefaults.ReadFile(filepath.Join("playbooks", file))
		if err != nil {
			return fmt.Errorf("read embedded default %q: %w", file, err)
		}
		path := filepath.Join(dir, file)
		if err := os.WriteFile(path, content, 0o644); err != nil {
			return fmt.Errorf("write default %q: %w", path, err)
		}
	}
	return nil
}

func loadDir(dir string) (map[string]*Playbook, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read playbooks dir: %w", err)
	}

	loaded := make(map[string]*Playbook)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := strings.ToLower(entry.Name())
		if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		pb, err := loadFile(path)
		if err != nil {
			return nil, err
		}
		if pb.ID == "" {
			pb.ID = strings.TrimSuffix(strings.TrimSuffix(strings.ToLower(entry.Name()), ".yaml"), ".yml")
		}
		if _, exists := loaded[pb.ID]; exists {
			return nil, fmt.Errorf("duplicate playbook id %q", pb.ID)
		}
		if err := normalizeAndValidate(pb); err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		loaded[pb.ID] = pb
	}
	return loaded, nil
}

func loadFile(path string) (*Playbook, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read playbook %q: %w", path, err)
	}
	var pb Playbook
	if err := yaml.Unmarshal(data, &pb); err != nil {
		return nil, fmt.Errorf("parse playbook %q: %w", path, err)
	}
	return &pb, nil
}

func normalizeAndValidate(pb *Playbook) error {
	if pb == nil {
		return fmt.Errorf("%w: playbook is required", ErrInvalidPlaybook)
	}
	pb.ID = strings.TrimSpace(strings.ToLower(pb.ID))
	if err := validateID(pb.ID); err != nil {
		return err
	}
	pb.Name = strings.TrimSpace(pb.Name)
	if pb.Name == "" {
		return fmt.Errorf("%w: name is required", ErrInvalidPlaybook)
	}
	pb.Description = strings.TrimSpace(pb.Description)

	if pb.Phases == nil {
		pb.Phases = []Phase{}
	}
	for i := range pb.Phases {
		pb.Phases[i].Name = strings.TrimSpace(pb.Phases[i].Name)
		pb.Phases[i].Agent = strings.TrimSpace(pb.Phases[i].Agent)
		pb.Phases[i].Role = strings.TrimSpace(pb.Phases[i].Role)
		pb.Phases[i].Description = strings.TrimSpace(pb.Phases[i].Description)
		if pb.Phases[i].Name == "" {
			return fmt.Errorf("%w: phase[%d].name is required", ErrInvalidPlaybook, i)
		}
		if pb.Phases[i].Agent == "" {
			return fmt.Errorf("%w: phase[%d].agent is required", ErrInvalidPlaybook, i)
		}
		if pb.Phases[i].Role == "" {
			return fmt.Errorf("%w: phase[%d].role is required", ErrInvalidPlaybook, i)
		}
	}

	if err := normalizeWorkflow(pb); err != nil {
		return err
	}

	return nil
}

func validateID(id string) error {
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("%w: id is required", ErrInvalidPlaybook)
	}
	if !playbookIDPattern.MatchString(id) {
		return fmt.Errorf("%w: id must be lowercase alphanumeric with hyphens", ErrInvalidPlaybook)
	}
	return nil
}

func clone(pb *Playbook) *Playbook {
	if pb == nil {
		return nil
	}
	out := *pb
	out.Phases = append([]Phase(nil), pb.Phases...)
	out.Workflow = cloneWorkflow(pb.Workflow)
	return &out
}

func normalizeWorkflow(pb *Playbook) error {
	if pb == nil {
		return fmt.Errorf("%w: playbook is required", ErrInvalidPlaybook)
	}
	if workflowIsEmpty(pb.Workflow) {
		pb.Workflow = workflowFromLegacyPhases(pb.Phases)
	}
	if workflowIsEmpty(pb.Workflow) {
		return fmt.Errorf("%w: workflow requires at least one stage role", ErrInvalidPlaybook)
	}
	if err := normalizeStage("plan", &pb.Workflow.Plan); err != nil {
		return err
	}
	if err := normalizeStage("build", &pb.Workflow.Build); err != nil {
		return err
	}
	if err := normalizeStage("test", &pb.Workflow.Test); err != nil {
		return err
	}
	return nil
}

func normalizeStage(stageName string, stage *Stage) error {
	if stage == nil {
		return fmt.Errorf("%w: workflow.%s is required", ErrInvalidPlaybook, stageName)
	}
	if stage.Roles == nil {
		stage.Roles = []StageRole{}
	}
	if !stage.Enabled && len(stage.Roles) == 0 {
		return nil
	}
	if len(stage.Roles) == 0 && stage.Enabled {
		return fmt.Errorf("%w: workflow.%s.enabled requires at least one role", ErrInvalidPlaybook, stageName)
	}
	for i := range stage.Roles {
		stage.Roles[i].Name = strings.TrimSpace(stage.Roles[i].Name)
		stage.Roles[i].Responsibilities = strings.TrimSpace(stage.Roles[i].Responsibilities)
		stage.Roles[i].SuggestedPrompt = strings.TrimSpace(stage.Roles[i].SuggestedPrompt)
		if stage.Roles[i].AllowedAgents == nil {
			stage.Roles[i].AllowedAgents = []string{}
		}
		for j := range stage.Roles[i].AllowedAgents {
			stage.Roles[i].AllowedAgents[j] = strings.TrimSpace(stage.Roles[i].AllowedAgents[j])
		}
		stage.Roles[i].AllowedAgents = compactNonEmpty(stage.Roles[i].AllowedAgents)
		if stage.Roles[i].Name == "" {
			return fmt.Errorf("%w: workflow.%s.roles[%d].name is required", ErrInvalidPlaybook, stageName, i)
		}
		if stage.Roles[i].Responsibilities == "" {
			return fmt.Errorf("%w: workflow.%s.roles[%d].responsibilities is required", ErrInvalidPlaybook, stageName, i)
		}
		if len(stage.Roles[i].AllowedAgents) == 0 {
			return fmt.Errorf("%w: workflow.%s.roles[%d].allowed_agents is required", ErrInvalidPlaybook, stageName, i)
		}
	}
	return nil
}

func compactNonEmpty(values []string) []string {
	if len(values) == 0 {
		return values
	}
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if _, exists := seen[v]; exists {
			continue
		}
		seen[v] = struct{}{}
		result = append(result, v)
	}
	return result
}

func workflowIsEmpty(workflow Workflow) bool {
	return len(workflow.Plan.Roles) == 0 && len(workflow.Build.Roles) == 0 && len(workflow.Test.Roles) == 0
}

func workflowFromLegacyPhases(phases []Phase) Workflow {
	workflow := Workflow{
		Plan:  Stage{Enabled: true, Roles: []StageRole{}},
		Build: Stage{Enabled: true, Roles: []StageRole{}},
		Test:  Stage{Enabled: true, Roles: []StageRole{}},
	}
	for _, phase := range phases {
		role := StageRole{
			Name:             strings.TrimSpace(phase.Role),
			Responsibilities: strings.TrimSpace(phase.Description),
			AllowedAgents:    compactNonEmpty([]string{strings.TrimSpace(phase.Agent)}),
		}
		if role.Name == "" {
			role.Name = strings.TrimSpace(phase.Name)
		}
		if role.Responsibilities == "" {
			role.Responsibilities = fmt.Sprintf("Run legacy phase %q", strings.TrimSpace(phase.Name))
		}
		target := stageForLegacyPhase(phase)
		switch target {
		case "plan":
			workflow.Plan.Roles = append(workflow.Plan.Roles, role)
		case "test":
			workflow.Test.Roles = append(workflow.Test.Roles, role)
		default:
			workflow.Build.Roles = append(workflow.Build.Roles, role)
		}
	}
	workflow.Plan.Enabled = len(workflow.Plan.Roles) > 0
	workflow.Build.Enabled = len(workflow.Build.Roles) > 0
	workflow.Test.Enabled = len(workflow.Test.Roles) > 0
	return workflow
}

func stageForLegacyPhase(phase Phase) string {
	role := strings.ToLower(strings.TrimSpace(phase.Role))
	name := strings.ToLower(strings.TrimSpace(phase.Name))
	switch {
	case strings.Contains(role, "plan"), strings.Contains(role, "architect"), strings.Contains(role, "research"), strings.Contains(name, "discover"), strings.Contains(name, "plan"):
		return "plan"
	case strings.Contains(role, "test"), strings.Contains(role, "review"), strings.Contains(role, "qa"), strings.Contains(name, "verify"), strings.Contains(name, "review"), strings.Contains(name, "test"):
		return "test"
	default:
		return "build"
	}
}

func cloneWorkflow(workflow Workflow) Workflow {
	return Workflow{
		Plan:  cloneStage(workflow.Plan),
		Build: cloneStage(workflow.Build),
		Test:  cloneStage(workflow.Test),
	}
}

func cloneStage(stage Stage) Stage {
	out := Stage{
		Enabled: stage.Enabled,
		Roles:   make([]StageRole, 0, len(stage.Roles)),
	}
	for _, role := range stage.Roles {
		out.Roles = append(out.Roles, StageRole{
			Name:             role.Name,
			Responsibilities: role.Responsibilities,
			AllowedAgents:    append([]string(nil), role.AllowedAgents...),
			SuggestedPrompt:  role.SuggestedPrompt,
		})
	}
	return out
}
