package tasks

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Task represents a single task stored as a markdown file with YAML frontmatter.
type Task struct {
	ID          string
	Title       string
	Description string
	Status      string   // "backlog", "in-progress", "done"
	Labels      []string
	Branch      string   // linked branch name
	Commits     []string // linked commit hashes
	Created     string   // ISO 8601
	Updated     string   // ISO 8601
	Priority    int      // 0=none, 1=low, 2=medium, 3=high
}

// tasksDir returns the .jock/tasks/ directory path for a repo, creating it if needed.
func tasksDir(repoPath string) (string, error) {
	dir := filepath.Join(repoPath, ".jock", "tasks")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("create tasks dir: %w", err)
	}
	return dir, nil
}

// nextID scans existing task files to determine the next sequential ID.
func nextID(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "001", nil
	}
	maxID := 0
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		parts := strings.SplitN(e.Name(), "-", 2)
		if len(parts) >= 1 {
			n, _ := strconv.Atoi(parts[0])
			if n > maxID {
				maxID = n
			}
		}
	}
	return fmt.Sprintf("%03d", maxID+1), nil
}

// slugify converts a title to a URL-safe slug.
func slugify(title string) string {
	re := regexp.MustCompile(`[^a-z0-9]+`)
	slug := re.ReplaceAllString(strings.ToLower(title), "-")
	slug = strings.Trim(slug, "-")
	if len(slug) > 40 {
		slug = slug[:40]
		slug = strings.TrimRight(slug, "-")
	}
	return slug
}

// filename returns the markdown filename for a task.
func filename(id, title string) string {
	return fmt.Sprintf("%s-%s.md", id, slugify(title))
}

// marshalTask converts a Task to markdown with YAML frontmatter.
func marshalTask(t *Task) []byte {
	var sb strings.Builder
	sb.WriteString("---\n")
	sb.WriteString(fmt.Sprintf("id: %s\n", t.ID))
	sb.WriteString(fmt.Sprintf("title: %s\n", t.Title))
	sb.WriteString(fmt.Sprintf("status: %s\n", t.Status))
	sb.WriteString(fmt.Sprintf("priority: %d\n", t.Priority))
	if len(t.Labels) > 0 {
		sb.WriteString(fmt.Sprintf("labels: [%s]\n", strings.Join(t.Labels, ", ")))
	}
	if t.Branch != "" {
		sb.WriteString(fmt.Sprintf("branch: %s\n", t.Branch))
	}
	if len(t.Commits) > 0 {
		sb.WriteString(fmt.Sprintf("commits: [%s]\n", strings.Join(t.Commits, ", ")))
	}
	sb.WriteString(fmt.Sprintf("created: %s\n", t.Created))
	sb.WriteString(fmt.Sprintf("updated: %s\n", t.Updated))
	sb.WriteString("---\n\n")
	sb.WriteString(t.Description)
	sb.WriteString("\n")
	return []byte(sb.String())
}

// parseTask parses a markdown file with YAML frontmatter into a Task.
func parseTask(data []byte) (*Task, error) {
	content := string(data)
	if !strings.HasPrefix(content, "---\n") {
		return nil, fmt.Errorf("missing frontmatter")
	}
	end := strings.Index(content[4:], "\n---\n")
	if end < 0 {
		return nil, fmt.Errorf("unterminated frontmatter")
	}
	frontmatter := content[4 : 4+end]
	body := strings.TrimSpace(content[4+end+5:])

	t := &Task{
		Status:      "backlog",
		Description: body,
	}

	for _, line := range strings.Split(frontmatter, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		idx := strings.Index(line, ": ")
		if idx < 0 {
			continue
		}
		key := line[:idx]
		val := line[idx+2:]

		switch key {
		case "id":
			t.ID = val
		case "title":
			t.Title = val
		case "status":
			t.Status = val
		case "priority":
			t.Priority, _ = strconv.Atoi(val)
		case "labels":
			t.Labels = parseList(val)
		case "branch":
			t.Branch = val
		case "commits":
			t.Commits = parseList(val)
		case "created":
			t.Created = val
		case "updated":
			t.Updated = val
		}
	}
	return t, nil
}

// parseList parses "[a, b, c]" into []string.
func parseList(s string) []string {
	s = strings.TrimPrefix(s, "[")
	s = strings.TrimSuffix(s, "]")
	if s == "" {
		return nil
	}
	var result []string
	for _, item := range strings.Split(s, ",") {
		item = strings.TrimSpace(item)
		if item != "" {
			result = append(result, item)
		}
	}
	return result
}

// List returns all tasks in the repo, optionally filtered by status.
func List(repoPath, statusFilter string) ([]Task, error) {
	dir, err := tasksDir(repoPath)
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil // no tasks yet
	}

	var tasks []Task
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		t, err := parseTask(data)
		if err != nil {
			continue
		}
		if statusFilter != "" && t.Status != statusFilter {
			continue
		}
		tasks = append(tasks, *t)
	}

	// Sort by priority desc, then by ID asc
	sort.Slice(tasks, func(i, j int) bool {
		if tasks[i].Priority != tasks[j].Priority {
			return tasks[i].Priority > tasks[j].Priority
		}
		return tasks[i].ID < tasks[j].ID
	})

	return tasks, nil
}

// Create creates a new task and writes it to disk.
func Create(repoPath, title, description string, labels []string, priority int) (*Task, error) {
	dir, err := tasksDir(repoPath)
	if err != nil {
		return nil, err
	}

	id, err := nextID(dir)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC().Format(time.RFC3339)
	t := &Task{
		ID:          id,
		Title:       title,
		Description: description,
		Status:      "backlog",
		Labels:      labels,
		Priority:    priority,
		Created:     now,
		Updated:     now,
	}

	path := filepath.Join(dir, filename(id, title))
	if err := os.WriteFile(path, marshalTask(t), 0644); err != nil {
		return nil, fmt.Errorf("write task: %w", err)
	}
	return t, nil
}

// findTaskFile finds the file for a given task ID.
func findTaskFile(dir, id string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), id+"-") && strings.HasSuffix(e.Name(), ".md") {
			return filepath.Join(dir, e.Name()), nil
		}
	}
	return "", fmt.Errorf("task %s not found", id)
}

// Get returns a single task by ID.
func Get(repoPath, id string) (*Task, error) {
	dir, err := tasksDir(repoPath)
	if err != nil {
		return nil, err
	}
	path, err := findTaskFile(dir, id)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return parseTask(data)
}

// Update modifies an existing task.
func Update(repoPath, id, title, description, status string, labels []string, branch string, priority int) (*Task, error) {
	dir, err := tasksDir(repoPath)
	if err != nil {
		return nil, err
	}

	oldPath, err := findTaskFile(dir, id)
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(oldPath)
	if err != nil {
		return nil, err
	}

	t, err := parseTask(data)
	if err != nil {
		return nil, err
	}

	// Apply updates (only non-zero values)
	if title != "" {
		t.Title = title
	}
	if description != "" {
		t.Description = description
	}
	if status != "" {
		t.Status = status
	}
	if labels != nil {
		t.Labels = labels
	}
	if branch != "" {
		t.Branch = branch
	}
	if priority >= 0 {
		t.Priority = priority
	}
	t.Updated = time.Now().UTC().Format(time.RFC3339)

	// If title changed, rename the file
	newPath := filepath.Join(dir, filename(t.ID, t.Title))
	if oldPath != newPath {
		os.Remove(oldPath)
	}

	if err := os.WriteFile(newPath, marshalTask(t), 0644); err != nil {
		return nil, fmt.Errorf("write task: %w", err)
	}
	return t, nil
}

// Delete removes a task file.
func Delete(repoPath, id string) error {
	dir, err := tasksDir(repoPath)
	if err != nil {
		return err
	}
	path, err := findTaskFile(dir, id)
	if err != nil {
		return err
	}
	return os.Remove(path)
}
