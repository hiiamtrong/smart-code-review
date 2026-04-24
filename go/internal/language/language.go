// Package language detects the primary programming language of a code change.
package language

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// Known language identifiers returned by this package.
const (
	TypeScript  = "typescript"
	JavaScript  = "javascript"
	Python      = "python"
	Java        = "java"
	Go          = "go"
	CSharp      = "csharp"
	Ruby        = "ruby"
	PHP         = "php"
	C           = "c"
	Cpp         = "cpp"
	Rust        = "rust"
	Kotlin      = "kotlin"
	Swift       = "swift"
	Scala       = "scala"
	Dart        = "dart"
	Shell       = "shell"
	YAML        = "yaml"
	SQL         = "sql"
	HTML        = "html"
	CSS         = "css"
	SCSS        = "scss"
	Vue         = "vue"
	Svelte      = "svelte"
	Unknown     = "plaintext"
)

// extLanguage maps file extensions to language identifiers.
var extLanguage = []struct {
	ext  string
	lang string
}{
	{".ts", TypeScript},
	{".tsx", TypeScript},
	{".js", JavaScript},
	{".jsx", JavaScript},
	{".mjs", JavaScript},
	{".cjs", JavaScript},
	{".py", Python},
	{".pyw", Python},
	{".java", Java},
	{".go", Go},
	{".cs", CSharp},
	{".rb", Ruby},
	{".php", PHP},
	{".c", C},
	{".h", C},
	{".cpp", Cpp},
	{".cc", Cpp},
	{".cxx", Cpp},
	{".hpp", Cpp},
	{".hxx", Cpp},
	{".rs", Rust},
	{".kt", Kotlin},
	{".kts", Kotlin},
	{".swift", Swift},
	{".scala", Scala},
	{".dart", Dart},
	{".sh", Shell},
	{".bash", Shell},
	{".zsh", Shell},
	{".yml", YAML},
	{".yaml", YAML},
	{".sql", SQL},
	{".html", HTML},
	{".htm", HTML},
	{".css", CSS},
	{".scss", SCSS},
	{".sass", SCSS},
	{".less", CSS},
	{".vue", Vue},
	{".svelte", Svelte},
}

// DetectFromDiff infers the language(s) by scanning file extensions
// mentioned in unified diff headers ("diff --git a/X b/X").
// Returns all detected languages joined by comma (e.g. "typescript,python"),
// or "plaintext" if none matched.
func DetectFromDiff(diff string) string {
	seen := make(map[string]bool)
	var langs []string
	for _, el := range extLanguage {
		if containsExtension(diff, el.ext) && !seen[el.lang] {
			seen[el.lang] = true
			langs = append(langs, el.lang)
		}
	}
	if len(langs) == 0 {
		return Unknown
	}
	return strings.Join(langs, ",")
}

// DetectFromProject infers the primary language from project marker files
// in the given directory. Used in CI context where the diff may not be available.
func DetectFromProject(root string) string {
	switch {
	case fileExists(root, "package.json"):
		if isTypeScriptProject(filepath.Join(root, "package.json")) {
			return TypeScript
		}
		return JavaScript
	case fileExists(root, "requirements.txt"), fileExists(root, "pyproject.toml"), fileExists(root, "setup.py"):
		return Python
	case fileExists(root, "pom.xml"), fileExists(root, "build.gradle"), fileExists(root, "build.gradle.kts"):
		return Java
	case fileExists(root, "go.mod"):
		return Go
	case globExists(root, "*.csproj"), globExists(root, "*.sln"):
		return CSharp
	case fileExists(root, "Gemfile"):
		return Ruby
	case fileExists(root, "composer.json"):
		return PHP
	default:
		return Unknown
	}
}

// ─── internal helpers ─────────────────────────────────────────────────────────

// containsExtension checks whether the diff text references any file with ext.
func containsExtension(diff, ext string) bool {
	// Check both "diff --git" headers and "+++ b/" lines
	return strings.Contains(diff, ext+"\n") ||
		strings.Contains(diff, ext+" ") ||
		strings.Contains(diff, ext+"\r")
}

func fileExists(root, name string) bool {
	_, err := os.Stat(filepath.Join(root, name))
	return err == nil
}

func globExists(root, pattern string) bool {
	matches, err := filepath.Glob(filepath.Join(root, pattern))
	return err == nil && len(matches) > 0
}

// isTypeScriptProject checks if package.json lists typescript as a dependency.
func isTypeScriptProject(pkgPath string) bool {
	data, err := os.ReadFile(pkgPath)
	if err != nil {
		return false
	}
	var pkg struct {
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		// Fall back to string search
		return strings.Contains(string(data), `"typescript"`)
	}
	_, inDeps := pkg.Dependencies["typescript"]
	_, inDev := pkg.DevDependencies["typescript"]
	return inDeps || inDev
}
