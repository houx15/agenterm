package orchestrator

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

var skillNamePattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

var defaultSkillRoots = []string{
	"skills",
	".agents/skills",
	".claude/skills",
}

type SkillSpec struct {
	ID          string
	Name        string
	Description string
	Details     string
	Path        string
}

type skillFrontmatter struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

func listSkillSpecs() []SkillSpec {
	root, err := os.Getwd()
	if err != nil {
		return nil
	}
	loaded := map[string]SkillSpec{}
	for _, rootDir := range discoverSkillRootDirs(root) {
		items, err := os.ReadDir(rootDir)
		if err != nil {
			continue
		}
		for _, item := range items {
			if !item.IsDir() {
				continue
			}
			dirName := strings.TrimSpace(item.Name())
			if !skillNamePattern.MatchString(dirName) {
				continue
			}
			skillPath := filepath.Join(rootDir, dirName, "SKILL.md")
			spec, ok := parseSkillFile(skillPath, dirName)
			if !ok {
				continue
			}
			if _, exists := loaded[spec.ID]; exists {
				continue
			}
			loaded[spec.ID] = spec
		}
	}
	skills := make([]SkillSpec, 0, len(loaded))
	for _, spec := range loaded {
		skills = append(skills, spec)
	}
	sort.Slice(skills, func(i, j int) bool {
		if skills[i].Name == skills[j].Name {
			return skills[i].ID < skills[j].ID
		}
		return skills[i].Name < skills[j].Name
	})
	return skills
}

func discoverSkillRootDirs(start string) []string {
	start = strings.TrimSpace(start)
	if start == "" {
		return nil
	}
	out := []string{}
	seen := map[string]struct{}{}
	current := start
	for {
		for _, relRoot := range defaultSkillRoots {
			candidate := filepath.Join(current, relRoot)
			clean := filepath.Clean(candidate)
			if _, ok := seen[clean]; ok {
				continue
			}
			seen[clean] = struct{}{}
			out = append(out, clean)
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}
	return out
}

func parseSkillFile(path string, dirName string) (SkillSpec, bool) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return SkillSpec{}, false
	}
	content := string(raw)
	fm, body := splitFrontmatter(content)
	if strings.TrimSpace(fm) == "" || strings.TrimSpace(body) == "" {
		return SkillSpec{}, false
	}
	var meta skillFrontmatter
	if err := yaml.Unmarshal([]byte(fm), &meta); err != nil {
		return SkillSpec{}, false
	}
	id := strings.TrimSpace(strings.ToLower(meta.Name))
	if id == "" {
		id = strings.TrimSpace(strings.ToLower(dirName))
	}
	if !skillNamePattern.MatchString(id) || !strings.EqualFold(id, dirName) {
		return SkillSpec{}, false
	}
	name := strings.TrimSpace(meta.Name)
	if name == "" {
		name = id
	}
	desc := strings.TrimSpace(meta.Description)
	if desc == "" {
		return SkillSpec{}, false
	}
	return SkillSpec{
		ID:          id,
		Name:        name,
		Description: desc,
		Details:     strings.TrimSpace(body),
		Path:        path,
	}, true
}

func splitFrontmatter(content string) (string, string) {
	trimmed := strings.TrimLeft(content, "\ufeff\r\n\t ")
	if !strings.HasPrefix(trimmed, "---\n") {
		return "", content
	}
	rest := strings.TrimPrefix(trimmed, "---\n")
	idx := strings.Index(rest, "\n---\n")
	if idx < 0 {
		return "", content
	}
	fm := rest[:idx]
	body := rest[idx+len("\n---\n"):]
	return fm, body
}

func SkillSummaries() []map[string]string {
	skills := listSkillSpecs()
	out := make([]map[string]string, 0, len(skills))
	for _, spec := range skills {
		out = append(out, map[string]string{
			"id":          spec.ID,
			"name":        spec.Name,
			"description": spec.Description,
		})
	}
	return out
}

func SkillDetailsByID(id string) (*SkillSpec, bool) {
	needle := strings.TrimSpace(strings.ToLower(id))
	if needle == "" {
		return nil, false
	}
	skills := listSkillSpecs()
	for i := range skills {
		if strings.EqualFold(skills[i].ID, needle) {
			return &skills[i], true
		}
	}
	return nil, false
}

func SkillSummaryPromptBlock() string {
	skills := SkillSummaries()
	if len(skills) == 0 {
		return ""
	}
	lines := make([]string, 0, len(skills)+2)
	lines = append(lines, "Available skills (progressive disclosure):")
	for _, skill := range skills {
		lines = append(lines, fmt.Sprintf("- %s: %s", skill["id"], skill["description"]))
	}
	lines = append(lines, "Call get_skill_details(skill_id) only when you decide to apply that skill.")
	return strings.Join(lines, "\n")
}
