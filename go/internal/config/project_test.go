package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	testProjectPath    = "/home/user/project"
	testModelGPT4o     = "gpt-4o"
	fmtListProjectsErr = "ListProjects: %v"
)

func TestProjectID_Deterministic(t *testing.T) {
	id1 := ProjectID(testProjectPath)
	id2 := ProjectID(testProjectPath)
	if id1 != id2 {
		t.Errorf("ProjectID not deterministic: %q != %q", id1, id2)
	}
}

func TestProjectID_DifferentPaths(t *testing.T) {
	id1 := ProjectID("/home/user/projectA")
	id2 := ProjectID("/home/user/projectB")
	if id1 == id2 {
		t.Errorf("different paths should produce different IDs: both %q", id1)
	}
}

func TestProjectID_Length(t *testing.T) {
	id := ProjectID("/some/path")
	if len(id) != 12 {
		t.Errorf("ProjectID length: got %d, want 12", len(id))
	}
}

func TestProjectID_TrailingSlash(t *testing.T) {
	// filepath.Clean removes trailing slashes, so these should match.
	id1 := ProjectID(testProjectPath)
	id2 := ProjectID("/home/user/project/")
	if id1 != id2 {
		t.Errorf("trailing slash should be normalised: %q != %q", id1, id2)
	}
}

func TestWritePartialConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config")

	m := map[string]string{
		"AI_MODEL":    testModelGPT4o,
		"AI_PROVIDER": "openai",
	}

	if err := writePartialConfig(path, m); err != nil {
		t.Fatalf("writePartialConfig: %v", err)
	}

	// Read back.
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f.Close()

	got, err := parseShellConfig(f)
	if err != nil {
		t.Fatalf("parseShellConfig: %v", err)
	}
	if got["AI_MODEL"] != testModelGPT4o {
		t.Errorf("AI_MODEL: got %q, want %q", got["AI_MODEL"], testModelGPT4o)
	}
	if got["AI_PROVIDER"] != "openai" {
		t.Errorf("AI_PROVIDER: got %q, want %q", got["AI_PROVIDER"], "openai")
	}
	// Should NOT have other keys.
	if _, ok := got["ENABLE_AI_REVIEW"]; ok {
		t.Error("partial config should not contain un-set keys")
	}
}

func TestListProjects_Empty(t *testing.T) {
	dir := t.TempDir()
	setTestHome(t, dir)

	projects, err := ListProjects()
	if err != nil {
		t.Fatalf(fmtListProjectsErr, err)
	}
	if len(projects) != 0 {
		t.Errorf("expected empty list, got %d projects", len(projects))
	}
}

func TestListProjects_MultipleProjects(t *testing.T) {
	dir := t.TempDir()
	setTestHome(t, dir)

	projectsDir := filepath.Join(dir, testConfigSubdir, testAppName, "projects")

	// Create two project dirs.
	for _, id := range []string{"aaa111bbb222", "ccc333ddd444"} {
		pDir := filepath.Join(projectsDir, id)
		if err := os.MkdirAll(pDir, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(pDir, "config"), []byte("AI_MODEL=\""+testModelGPT4o+"\"\n"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(pDir, "repo-path"), []byte("/some/repo/"+id+"\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}

	projects, err := ListProjects()
	if err != nil {
		t.Fatalf(fmtListProjectsErr, err)
	}
	if len(projects) != 2 {
		t.Fatalf("expected 2 projects, got %d", len(projects))
	}

	// Verify fields.
	for _, p := range projects {
		if p.ID == "" || p.RepoPath == "" || p.ConfigPath == "" {
			t.Errorf("incomplete project info: %+v", p)
		}
	}
}

func TestListProjects_SkipsDirWithoutConfig(t *testing.T) {
	dir := t.TempDir()
	setTestHome(t, dir)

	projectsDir := filepath.Join(dir, testConfigSubdir, testAppName, "projects")

	// Create a dir without a config file.
	pDir := filepath.Join(projectsDir, "noconfig123")
	if err := os.MkdirAll(pDir, 0700); err != nil {
		t.Fatal(err)
	}

	projects, err := ListProjects()
	if err != nil {
		t.Fatalf(fmtListProjectsErr, err)
	}
	if len(projects) != 0 {
		t.Errorf("expected 0 projects (no config file), got %d", len(projects))
	}
}

func TestListProjects_MissingRepoPath(t *testing.T) {
	dir := t.TempDir()
	setTestHome(t, dir)

	projectsDir := filepath.Join(dir, testConfigSubdir, testAppName, "projects")
	pDir := filepath.Join(projectsDir, "abc123def456")
	if err := os.MkdirAll(pDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pDir, "config"), []byte(`AI_MODEL="x"`+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	// No repo-path file.

	projects, err := ListProjects()
	if err != nil {
		t.Fatalf(fmtListProjectsErr, err)
	}
	if len(projects) != 1 {
		t.Fatalf("expected 1 project, got %d", len(projects))
	}
	if projects[0].RepoPath != "" {
		t.Errorf("expected empty RepoPath, got %q", projects[0].RepoPath)
	}
}

func TestRemoveProject_Exists(t *testing.T) {
	dir := t.TempDir()
	setTestHome(t, dir)

	projectsDir := filepath.Join(dir, testConfigSubdir, testAppName, "projects")
	id := "remove123456"
	pDir := filepath.Join(projectsDir, id)
	if err := os.MkdirAll(pDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pDir, "config"), []byte(""), 0600); err != nil {
		t.Fatal(err)
	}

	if err := RemoveProject(id); err != nil {
		t.Fatalf("RemoveProject: %v", err)
	}

	if _, err := os.Stat(pDir); !os.IsNotExist(err) {
		t.Error("project dir should be removed")
	}
}

func TestLoadProjectRaw_NoProjectConfig(t *testing.T) {
	// When inside a git repo but no project config exists, should return nil, nil.
	dir := t.TempDir()
	setTestHome(t, dir)

	m, err := LoadProjectRaw()
	if err != nil {
		t.Fatalf("LoadProjectRaw: %v", err)
	}
	// Either nil (no project config dir) or nil (file doesn't exist) is acceptable.
	if m != nil {
		t.Errorf("expected nil map when no project config, got %v", m)
	}
}

func TestSaveProjectField_AndLoadBack(t *testing.T) {
	// This test only works when run inside a git repo (which this project is).
	root := detectRepoRoot()
	if root == "" {
		t.Skip("not inside a git repo, skipping SaveProjectField test")
	}

	dir := t.TempDir()
	setTestHome(t, dir)

	// Save a field.
	if err := SaveProjectField("AI_MODEL", "test-model-xyz"); err != nil {
		t.Fatalf("SaveProjectField: %v", err)
	}

	// Verify the project config directory was created.
	projectDir, err := ProjectConfigDir()
	if err != nil {
		t.Fatalf("ProjectConfigDir: %v", err)
	}
	if projectDir == "" {
		t.Fatal("ProjectConfigDir returned empty string")
	}

	// Load it back.
	m, err := LoadProjectRaw()
	if err != nil {
		t.Fatalf("LoadProjectRaw: %v", err)
	}
	if m == nil {
		t.Fatal("LoadProjectRaw returned nil after SaveProjectField")
	}
	if m["AI_MODEL"] != "test-model-xyz" {
		t.Errorf("AI_MODEL: got %q, want %q", m["AI_MODEL"], "test-model-xyz")
	}

	// Save another field — should preserve the first.
	if err := SaveProjectField("AI_PROVIDER", "test-provider"); err != nil {
		t.Fatalf("SaveProjectField second key: %v", err)
	}
	m, err = LoadProjectRaw()
	if err != nil {
		t.Fatalf("LoadProjectRaw after second save: %v", err)
	}
	if m["AI_MODEL"] != "test-model-xyz" {
		t.Errorf("AI_MODEL should be preserved: got %q", m["AI_MODEL"])
	}
	if m["AI_PROVIDER"] != "test-provider" {
		t.Errorf("AI_PROVIDER: got %q, want %q", m["AI_PROVIDER"], "test-provider")
	}

	// Verify repo-path metadata file exists.
	repoPathFile := filepath.Join(projectDir, "repo-path")
	b, err := os.ReadFile(repoPathFile)
	if err != nil {
		t.Fatalf("read repo-path: %v", err)
	}
	if strings.TrimSpace(string(b)) == "" {
		t.Error("repo-path file should not be empty")
	}

	// Clean up: remove the project config.
	id := ProjectID(root)
	if err := RemoveProject(id); err != nil {
		t.Fatalf("RemoveProject cleanup: %v", err)
	}
}

func TestRemoveProject_NotFound(t *testing.T) {
	dir := t.TempDir()
	setTestHome(t, dir)

	err := RemoveProject("nonexistent12")
	if err == nil {
		t.Error("expected error for non-existent project")
	}
}
