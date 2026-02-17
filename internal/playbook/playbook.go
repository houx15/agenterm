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

var languageByExt = map[string]string{
	".go":    "go",
	".rs":    "rust",
	".py":    "python",
	".rb":    "ruby",
	".java":  "java",
	".kt":    "kotlin",
	".js":    "javascript",
	".jsx":   "javascript",
	".ts":    "typescript",
	".tsx":   "typescript",
	".php":   "php",
	".cs":    "csharp",
	".swift": "swift",
}

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

	langSet, patternSet := inspectProject(repoPath)
	var best *Playbook
	bestScore := -1

	for _, pb := range snapshot {
		score, ok := matchScore(pb, langSet, patternSet)
		if !ok {
			continue
		}
		if score > bestScore {
			bestScore = score
			best = pb
			continue
		}
		if score == bestScore && best != nil {
			if strings.Compare(pb.Name, best.Name) < 0 || (pb.Name == best.Name && strings.Compare(pb.ID, best.ID) < 0) {
				best = pb
			}
		}
	}

	if best != nil {
		return clone(best)
	}
	if fallback, ok := snapshot["default"]; ok {
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

func matchScore(pb *Playbook, languages map[string]struct{}, projectPatterns map[string]struct{}) (int, bool) {
	matchedLang := 0
	if len(pb.Match.Languages) > 0 {
		for _, language := range pb.Match.Languages {
			if _, ok := languages[strings.ToLower(strings.TrimSpace(language))]; ok {
				matchedLang++
			}
		}
		if matchedLang == 0 {
			return 0, false
		}
	}

	matchedPattern := 0
	if len(pb.Match.ProjectPatterns) > 0 {
		for _, rawPattern := range pb.Match.ProjectPatterns {
			pattern := strings.ToLower(strings.TrimSpace(rawPattern))
			if pattern == "" {
				continue
			}
			for candidate := range projectPatterns {
				if strings.Contains(candidate, pattern) {
					matchedPattern++
					break
				}
			}
		}
		if matchedPattern == 0 {
			return 0, false
		}
	}

	score := matchedLang*3 + matchedPattern*2
	if len(pb.Match.Languages) == 0 && len(pb.Match.ProjectPatterns) == 0 {
		score = 1
	}
	return score, true
}

func inspectProject(repoPath string) (map[string]struct{}, map[string]struct{}) {
	languages := map[string]struct{}{}
	patterns := map[string]struct{}{}

	repoPath = strings.TrimSpace(repoPath)
	if repoPath == "" {
		return languages, patterns
	}

	entries, err := os.ReadDir(repoPath)
	if err == nil {
		for _, entry := range entries {
			name := strings.ToLower(entry.Name())
			patterns[name] = struct{}{}
			if entry.IsDir() {
				patterns[name+"/"] = struct{}{}
			}
		}
	}

	walked := 0
	_ = filepath.WalkDir(repoPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		name := strings.ToLower(d.Name())
		if d.IsDir() {
			if name == ".git" || name == "node_modules" || strings.HasPrefix(name, ".worktrees") {
				return filepath.SkipDir
			}
			if walked > 3000 {
				return filepath.SkipDir
			}
			return nil
		}
		walked++
		if walked > 3000 {
			return errors.New("done")
		}

		rel, relErr := filepath.Rel(repoPath, path)
		if relErr == nil {
			patterns[strings.ToLower(filepath.ToSlash(rel))] = struct{}{}
		}

		ext := strings.ToLower(filepath.Ext(name))
		if lang, ok := languageByExt[ext]; ok {
			languages[lang] = struct{}{}
		}

		switch name {
		case "go.mod":
			languages["go"] = struct{}{}
		case "package.json":
			languages["javascript"] = struct{}{}
			languages["typescript"] = struct{}{}
		case "gemfile":
			languages["ruby"] = struct{}{}
		case "pyproject.toml", "requirements.txt":
			languages["python"] = struct{}{}
		}
		return nil
	})
	return languages, patterns
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

	for _, file := range []string{"default.yaml", "go-backend.yaml"} {
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
	pb.ParallelismStrategy = strings.TrimSpace(pb.ParallelismStrategy)
	if pb.ParallelismStrategy == "" {
		pb.ParallelismStrategy = "Balanced parallelism based on dependency boundaries."
	}

	if pb.Match.Languages == nil {
		pb.Match.Languages = []string{}
	}
	if pb.Match.ProjectPatterns == nil {
		pb.Match.ProjectPatterns = []string{}
	}
	for i := range pb.Match.Languages {
		pb.Match.Languages[i] = strings.ToLower(strings.TrimSpace(pb.Match.Languages[i]))
	}
	for i := range pb.Match.ProjectPatterns {
		pb.Match.ProjectPatterns[i] = strings.TrimSpace(pb.Match.ProjectPatterns[i])
	}

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
	out.Match = Match{
		Languages:       append([]string(nil), pb.Match.Languages...),
		ProjectPatterns: append([]string(nil), pb.Match.ProjectPatterns...),
	}
	out.Phases = append([]Phase(nil), pb.Phases...)
	return &out
}
