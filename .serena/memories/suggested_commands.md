# Suggested Commands

## Testing the Action
Since this is a GitHub Action, testing requires either:
- Push to a test repository and create a PR to trigger the action
- Use `act` (GitHub Actions local runner) if available
- Manual script execution with environment variables set

## Manual Script Testing
```bash
# Test language detection
bash scripts/detect-language.sh

# Test AI review (requires OPENAI_API_KEY)
export OPENAI_API_KEY="your-key"
export GITHUB_TOKEN="your-token"
bash scripts/ai-review.sh
```

## Development Commands
```bash
# Make scripts executable
chmod +x scripts/*.sh

# Validate action.yml syntax
# (GitHub provides online validator or use act)

# Test shell script syntax
bash -n scripts/detect-language.sh
bash -n scripts/ai-review.sh
```

## System Utilities (macOS/Darwin)
- `ls`, `cd`, `mkdir`, `chmod` - Standard file operations
- `git` - Version control operations
- `curl` - HTTP requests and downloads
- `jq` - JSON processing (may need installation: `brew install jq`)
- `grep`, `find` - Text and file searching

## Environment Setup
- Ensure `GITHUB_TOKEN` is available for reviewdog
- Set `OPENAI_API_KEY` for AI review functionality
- Scripts automatically install reviewdog to `$HOME/bin`