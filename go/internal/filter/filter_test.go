package filter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// sampleDiff is a minimal multi-file unified diff for testing.
const sampleDiff = `diff --git a/src/app.ts b/src/app.ts
index 111..222 100644
--- a/src/app.ts
+++ b/src/app.ts
@@ -1,2 +1,3 @@
 const x = 1
+const y = 2
 export {}
diff --git a/package-lock.json b/package-lock.json
index 333..444 100644
--- a/package-lock.json
+++ b/package-lock.json
@@ -1 +1,2 @@
 {}
+{"version":2}
diff --git a/node_modules/lodash/index.js b/node_modules/lodash/index.js
index 555..666 100644
--- a/node_modules/lodash/index.js
+++ b/node_modules/lodash/index.js
@@ -1 +1 @@
-old
+new
diff --git a/dist/bundle.min.js b/dist/bundle.min.js
index 777..888 100644
--- a/dist/bundle.min.js
+++ b/dist/bundle.min.js
@@ -1 +1 @@
-old
+new
`

func TestFilterDiff_NoPatterns(t *testing.T) {
	out, count := FilterDiff(sampleDiff, nil)
	if out != sampleDiff {
		t.Error("expected unchanged diff when no patterns")
	}
	if count != 0 {
		t.Errorf("expected 0 ignored, got %d", count)
	}
}

func TestFilterDiff_EmptyDiff(t *testing.T) {
	out, count := FilterDiff("", []string{"*.lock"})
	if out != "" {
		t.Errorf("expected empty output, got %q", out)
	}
	if count != 0 {
		t.Errorf("expected 0 ignored, got %d", count)
	}
}

func TestFilterDiff_SingleExtensionPattern(t *testing.T) {
	// "*.json" should match package-lock.json (via **/*.json prefix)
	out, count := FilterDiff(sampleDiff, []string{"*.json"})
	if count != 1 {
		t.Errorf("expected 1 ignored file, got %d", count)
	}
	if strings.Contains(out, "package-lock.json") {
		t.Error("package-lock.json should have been filtered out")
	}
	if !strings.Contains(out, "src/app.ts") {
		t.Error("src/app.ts should remain in output")
	}
}

func TestFilterDiff_DirectoryPattern(t *testing.T) {
	// "node_modules/" or "node_modules/**" should filter lodash
	out, count := FilterDiff(sampleDiff, []string{"node_modules/**"})
	if count != 1 {
		t.Errorf("expected 1 ignored file, got %d", count)
	}
	if strings.Contains(out, "node_modules") {
		t.Error("node_modules file should be filtered out")
	}
}

func TestFilterDiff_GlobMinJS(t *testing.T) {
	// "*.min.js" should match dist/bundle.min.js
	out, count := FilterDiff(sampleDiff, []string{"*.min.js"})
	if count != 1 {
		t.Errorf("expected 1 ignored file, got %d", count)
	}
	if strings.Contains(out, "bundle.min.js") {
		t.Error("bundle.min.js should be filtered out")
	}
}

func TestFilterDiff_MultiplePatterns(t *testing.T) {
	patterns := []string{"*.json", "node_modules/**", "*.min.js"}
	out, count := FilterDiff(sampleDiff, patterns)
	if count != 3 {
		t.Errorf("expected 3 ignored files, got %d", count)
	}
	if !strings.Contains(out, "src/app.ts") {
		t.Error("src/app.ts should be the only remaining file")
	}
	if strings.Contains(out, "package-lock.json") || strings.Contains(out, "node_modules") || strings.Contains(out, "bundle.min.js") {
		t.Error("all three ignored files should be absent from output")
	}
}

func TestFilterDiff_AllFilesIgnored(t *testing.T) {
	out, count := FilterDiff(sampleDiff, []string{"**/*"})
	if out != "" && strings.TrimSpace(out) != "" {
		// Allow trailing newline from last block flush
		lines := strings.Split(strings.TrimSpace(out), "\n")
		if len(lines) > 0 && lines[0] != "" {
			t.Errorf("expected empty output when all files ignored, got:\n%s", out)
		}
	}
	if count != 4 {
		t.Errorf("expected 4 ignored files, got %d", count)
	}
}

func TestFilterDiff_DoubleStarPattern(t *testing.T) {
	diff := `diff --git a/src/components/Button/index.tsx b/src/components/Button/index.tsx
index 000..111 100644
--- a/src/components/Button/index.tsx
+++ b/src/components/Button/index.tsx
@@ -1 +1 @@
-old
+new
diff --git a/src/utils/helpers.ts b/src/utils/helpers.ts
index 222..333 100644
--- a/src/utils/helpers.ts
+++ b/src/utils/helpers.ts
@@ -1 +1 @@
-old
+new
`
	// "src/components/**" should only match the Button component
	out, count := FilterDiff(diff, []string{"src/components/**"})
	if count != 1 {
		t.Errorf("expected 1 ignored, got %d", count)
	}
	if strings.Contains(out, "Button") {
		t.Error("Button component should be filtered")
	}
	if !strings.Contains(out, "helpers.ts") {
		t.Error("helpers.ts should remain")
	}
}

func TestLoadIgnorePatterns_FileNotExist(t *testing.T) {
	patterns, err := LoadIgnorePatterns("/nonexistent/.aireviewignore")
	if err != nil {
		t.Errorf("expected nil error for missing file, got: %v", err)
	}
	if len(patterns) != 0 {
		t.Errorf("expected empty patterns, got %v", patterns)
	}
}

func TestLoadIgnorePatterns_ParsesCorrectly(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".aireviewignore")
	content := "# comment\n*.lock\n\nnode_modules/\ndist/**\n"
	os.WriteFile(path, []byte(content), 0644)

	patterns, err := LoadIgnorePatterns(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"*.lock", "node_modules/", "dist/**"}
	if len(patterns) != len(want) {
		t.Fatalf("got %d patterns, want %d: %v", len(patterns), len(want), patterns)
	}
	for i, p := range want {
		if patterns[i] != p {
			t.Errorf("pattern[%d]: got %q, want %q", i, patterns[i], p)
		}
	}
}

func TestLoadIgnorePatterns_CRLFHandling(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".aireviewignore")
	// CRLF line endings (Windows format)
	os.WriteFile(path, []byte("*.lock\r\nnode_modules/\r\n"), 0644)

	patterns, err := LoadIgnorePatterns(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(patterns) != 2 {
		t.Fatalf("expected 2 patterns, got %d: %v", len(patterns), patterns)
	}
	if patterns[0] != "*.lock" {
		t.Errorf("pattern[0]: got %q, want *.lock (no CR)", patterns[0])
	}
}

func TestMatchesAny(t *testing.T) {
	tests := []struct {
		file     string
		patterns []string
		want     bool
	}{
		{"package-lock.json", []string{"*.lock"}, false}, // no match on extension name
		{"package.lock", []string{"*.lock"}, true},
		{"dir/package.lock", []string{"*.lock"}, true},       // via **/ prefix
		{"node_modules/foo.js", []string{"node_modules/**"}, true},
		{"src/app.ts", []string{"node_modules/**", "*.json"}, false},
		{"dist/bundle.min.js", []string{"**/*.min.js"}, true},
		{"README.md", []string{"*.md"}, true},
	}
	for _, tt := range tests {
		got := matchesAny(tt.file, tt.patterns)
		if got != tt.want {
			t.Errorf("matchesAny(%q, %v) = %v, want %v", tt.file, tt.patterns, got, tt.want)
		}
	}
}
