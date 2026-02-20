package orchestrator

import (
	"context"
	"fmt"
	"io"
	"net/http"
	neturl "net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

var skillNamePattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

var defaultSkillRoots = []string{
	"skills",
}

var skillDownloadClient = &http.Client{Timeout: 20 * time.Second}

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
	out := make([]string, 0, len(defaultSkillRoots))
	for _, relRoot := range defaultSkillRoots {
		out = append(out, filepath.Clean(filepath.Join(start, relRoot)))
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

func resolveInstallSkillRoot(start string) string {
	dirs := discoverSkillRootDirs(start)
	for _, dir := range dirs {
		if filepath.Base(dir) == "skills" {
			if info, err := os.Stat(dir); err == nil && info.IsDir() {
				return dir
			}
		}
	}
	if strings.TrimSpace(start) == "" {
		return "skills"
	}
	return filepath.Join(start, "skills")
}

func normalizeOnlineSkillURL(rawURL string) (string, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return "", fmt.Errorf("url is required")
	}
	if strings.HasPrefix(rawURL, "https://raw.githubusercontent.com/") {
		return rawURL, nil
	}
	u, err := neturl.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("invalid url: %w", err)
	}
	if !strings.EqualFold(u.Hostname(), "github.com") {
		return "", fmt.Errorf("only github.com and raw.githubusercontent.com skill urls are supported")
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 6 || !strings.EqualFold(parts[2], "tree") {
		return "", fmt.Errorf("expected github tree url like /org/repo/tree/branch/skills/<skill-id>")
	}
	org := parts[0]
	repo := parts[1]
	branch := parts[3]
	relativePath := strings.Join(parts[4:], "/")
	if !strings.HasPrefix(relativePath, "skills/") {
		return "", fmt.Errorf("url must point to a skill folder under skills/")
	}
	return fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/%s/SKILL.md", org, repo, branch, relativePath), nil
}

func InstallSkillFromURL(ctx context.Context, rawURL string, overwrite bool) (*SkillSpec, error) {
	normalizedURL, err := normalizeOnlineSkillURL(rawURL)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, normalizedURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := skillDownloadClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		return nil, fmt.Errorf("download skill failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	fm, markdown := splitFrontmatter(string(body))
	if strings.TrimSpace(fm) == "" || strings.TrimSpace(markdown) == "" {
		return nil, fmt.Errorf("invalid SKILL.md format: frontmatter and body are required")
	}
	var meta skillFrontmatter
	if err := yaml.Unmarshal([]byte(fm), &meta); err != nil {
		return nil, fmt.Errorf("invalid skill frontmatter: %w", err)
	}
	skillID := strings.TrimSpace(strings.ToLower(meta.Name))
	if !skillNamePattern.MatchString(skillID) {
		return nil, fmt.Errorf("invalid skill name: %q", meta.Name)
	}
	if strings.TrimSpace(meta.Description) == "" {
		return nil, fmt.Errorf("invalid skill description: required")
	}
	wd, _ := os.Getwd()
	root := resolveInstallSkillRoot(wd)
	targetDir := filepath.Join(root, skillID)
	targetPath := filepath.Join(targetDir, "SKILL.md")
	if !overwrite {
		if _, err := os.Stat(targetPath); err == nil {
			return nil, fmt.Errorf("skill %q already exists; set overwrite=true to replace", skillID)
		}
	}
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(targetPath, body, 0o644); err != nil {
		return nil, err
	}
	spec, ok := parseSkillFile(targetPath, skillID)
	if !ok {
		return nil, fmt.Errorf("installed skill failed local validation")
	}
	return &spec, nil
}
