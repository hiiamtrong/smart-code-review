package language

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectFromDiff(t *testing.T) {
	tests := []struct {
		name string
		diff string
		want string
	}{
		{
			name: "typescript tsx",
			diff: "diff --git a/src/App.tsx b/src/App.tsx\n+++ b/src/App.tsx",
			want: TypeScript,
		},
		{
			name: "typescript ts",
			diff: "diff --git a/src/index.ts b/src/index.ts\n+++ b/src/index.ts",
			want: TypeScript,
		},
		{
			name: "javascript jsx",
			diff: "diff --git a/src/app.jsx b/src/app.jsx\n+++ b/src/app.jsx",
			want: JavaScript,
		},
		{
			name: "multiple languages",
			diff: "diff --git a/App.tsx b/App.tsx\ndiff --git a/helper.py b/helper.py",
			want: "typescript,python",
		},
		{
			name: "typescript and javascript",
			diff: "diff --git a/App.tsx b/App.tsx\ndiff --git a/helper.js b/helper.js",
			want: "typescript,javascript",
		},
		{
			name: "dedup same language",
			diff: "diff --git a/a.ts b/a.ts\ndiff --git a/b.tsx b/b.tsx",
			want: TypeScript,
		},
		{
			name: "c and rust",
			diff: "diff --git a/main.rs b/main.rs\ndiff --git a/util.c b/util.c",
			want: "c,rust",
		},
		{
			name: "vue file",
			diff: "diff --git a/App.vue b/App.vue",
			want: Vue,
		},
		{
			name: "shell script",
			diff: "diff --git a/deploy.sh b/deploy.sh",
			want: Shell,
		},
		{
			name: "yaml config",
			diff: "diff --git a/.github/workflows/ci.yml b/.github/workflows/ci.yml",
			want: YAML,
		},
		{
			name: "python",
			diff: "diff --git a/main.py b/main.py\n+++ b/main.py",
			want: Python,
		},
		{
			name: "java",
			diff: "diff --git a/Main.java b/Main.java\n+++ b/Main.java",
			want: Java,
		},
		{
			name: "go",
			diff: "diff --git a/main.go b/main.go\n+++ b/main.go",
			want: Go,
		},
		{
			name: "csharp",
			diff: "diff --git a/Program.cs b/Program.cs\n+++ b/Program.cs",
			want: CSharp,
		},
		{
			name: "ruby",
			diff: "diff --git a/app.rb b/app.rb\n+++ b/app.rb",
			want: Ruby,
		},
		{
			name: "php",
			diff: "diff --git a/index.php b/index.php\n+++ b/index.php",
			want: PHP,
		},
		{
			name: "unknown",
			diff: "diff --git a/Makefile b/Makefile\n+++ b/Makefile",
			want: "plaintext",
		},
		{
			name: "empty diff",
			diff: "",
			want: "plaintext",
		},
		{
			name: "ts only with js in content should not match js",
			diff: "diff --git a/app.ts b/app.ts\n+++ b/app.ts\n+import chart from 'chart.js';\n+const x = \"foo.js\";",
			want: TypeScript,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectFromDiff(tt.diff)
			if got != tt.want {
				t.Errorf("DetectFromDiff() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDetectFromProject(t *testing.T) {
	tests := []struct {
		name  string
		files map[string]string // filename -> content
		want  string
	}{
		{
			name: "typescript via package.json devDeps",
			files: map[string]string{
				"package.json": `{"devDependencies":{"typescript":"^5.0"}}`,
			},
			want: TypeScript,
		},
		{
			name: "javascript via package.json no ts",
			files: map[string]string{
				"package.json": `{"dependencies":{"react":"^18"}}`,
			},
			want: JavaScript,
		},
		{
			name: "python requirements.txt",
			files: map[string]string{"requirements.txt": "flask\n"},
			want:  Python,
		},
		{
			name: "python pyproject.toml",
			files: map[string]string{"pyproject.toml": "[tool.poetry]\n"},
			want:  Python,
		},
		{
			name: "java pom.xml",
			files: map[string]string{"pom.xml": "<project/>"},
			want:  Java,
		},
		{
			name: "go mod",
			files: map[string]string{"go.mod": "module example.com\ngo 1.22\n"},
			want:  Go,
		},
		{
			name: "ruby gemfile",
			files: map[string]string{"Gemfile": "source 'https://rubygems.org'\n"},
			want:  Ruby,
		},
		{
			name: "php composer",
			files: map[string]string{"composer.json": "{}"},
			want:  PHP,
		},
		{
			name:  "unknown empty dir",
			files: map[string]string{},
			want:  Unknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			for name, content := range tt.files {
				os.WriteFile(filepath.Join(dir, name), []byte(content), 0644)
			}
			got := DetectFromProject(dir)
			if got != tt.want {
				t.Errorf("DetectFromProject() = %q, want %q", got, tt.want)
			}
		})
	}
}
