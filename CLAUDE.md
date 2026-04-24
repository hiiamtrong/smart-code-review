# Smart Code Review

Pre-commit code review tool chạy AI + Semgrep + SonarQube analysis trên staged changes.

## Tech Stack

- **Go 1.25** — binary chính, build bằng goreleaser
- **Cobra** — CLI framework (`github.com/spf13/cobra`)
- **doublestar v4** — gitignore-style glob matching cho `.aireviewignore`
- **fatih/color** — terminal color output
- **goreleaser** — cross-platform build & GitHub Release
- **GitHub Actions** — CI/CD via composite action (`action.yml`)

External tools (không bundled, cần cài riêng):

- **Semgrep** — static analysis
- **SonarQube Scanner** — server-based analysis
- **reviewdog** — post diagnostics lên GitHub PR

## Project Structure

```text
├── go/                          # Go source code
│   ├── cmd/ai-review/           # Entry point (main.go)
│   ├── internal/
│   │   ├── cmd/                 # CLI commands (run-hook, ci-review, setup, config, install, update)
│   │   ├── config/              # Config loading & merging (env + YAML + git config)
│   │   ├── display/             # Terminal output formatting
│   │   ├── filter/              # .aireviewignore pattern matching (doublestar)
│   │   ├── gateway/             # AI Gateway HTTP client (SSE streaming + sync)
│   │   ├── git/                 # Git operations (diff, repo root, metadata)
│   │   ├── installer/           # Hook installation (pre-commit framework support)
│   │   ├── language/            # Language detection from diff & project files
│   │   ├── reviewdog/           # reviewdog integration & rdjson output
│   │   ├── semgrep/             # Semgrep scanner wrapper
│   │   ├── sonarqube/           # SonarQube analysis pipeline
│   │   └── updater/             # Self-update via GitHub Releases
│   ├── e2e/                     # End-to-end tests
│   └── Makefile                 # build, test, vet, lint, install
├── scripts/                     # Legacy bash scripts (migrated to Go)
├── docs/ai/                     # AI devkit phase docs
├── .goreleaser.yml              # Cross-platform build config
├── action.yml                   # GitHub Actions composite action
└── VERSION                      # Current version (synced by CI)
```

## Setup

```bash
# Prerequisites: Go 1.25+, Semgrep (optional), SonarQube Scanner (optional)

cd go
make install          # Build & install to ~/.local/bin/ai-review
ai-review setup       # Interactive config (AI Gateway, Semgrep, SonarQube)
ai-review install     # Install git pre-commit hook
```

## Testing & Running Tests

```bash
cd go

# Run all tests
make test
# Equivalent to: go test ./... -count=1 -timeout 120s

# Run specific package
go test ./internal/filter/ -v

# Run specific test
go test ./internal/cmd/ -v -run TestHookPrepareDiff -count=1

# Pre-push check (vet + test + build)
make check

# Coverage
go test ./... -coverprofile=coverage.out
go tool cover -func=coverage.out
```

Test conventions:

- File naming: `*_test.go` trong cùng package
- Test files cùng thư mục với source (không tách `test/` riêng)
- E2E tests: `go/e2e/e2e_test.go`

## Release

```bash
# 1. Bump version
echo "X.Y.Z" > VERSION
git add VERSION && git commit -m "chore: sync VERSION to X.Y.Z [skip ci]"

# 2. Tag
git tag vX.Y.Z

# 3. Push
git push origin main && git push origin vX.Y.Z

# 4. Build & publish (goreleaser)
GITHUB_TOKEN=$(gh auth token) goreleaser release --clean
```

Platforms: linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64.

## Key Conventions

- Config precedence: env vars > YAML config > git config
- `.aireviewignore` dùng gitignore-style glob syntax, filter file cho cả Semgrep, SonarQube và AI review
- Hook exit code 1 = block commit, 0 = allow
- Version injected at build time via `-ldflags`
