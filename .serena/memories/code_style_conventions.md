# Code Style & Conventions

## Shell Script Conventions
- Use `#!/usr/bin/env bash` shebang
- Set `set -e` for error handling (exit on error)
- Use meaningful variable names in UPPERCASE for environment variables
- Echo informative messages with emoji prefixes (🔎, ➡️)
- Use `$(dirname "$0")` for relative script paths

## File Structure Patterns
- Main action logic in `scripts/` directory
- Use descriptive script names (`detect-language.sh`, `ai-review.sh`)
- Action configuration in root `action.yml`

## Error Handling
- Scripts use `set -e` to exit on any command failure
- Curl commands use `-s` (silent) and `-f` (fail on HTTP errors)
- Language detection has fallback to "unknown" when no matches found

## API Integration Patterns
- Use curl for HTTP requests with proper headers
- Process JSON responses with `jq`
- Format outputs according to reviewdog's rdjson format
- Pipe outputs directly to reviewdog for processing

## Documentation Style
- Use emoji in console output for better UX
- Comment complex bash operations
- Maintain both English (README.md) and Vietnamese (REQUIREMENT.md) docs