# Tech Stack

## Core Technologies
- **Shell/Bash**: Primary scripting language for the action logic
- **GitHub Actions**: Composite action architecture using `action.yml`
- **Reviewdog**: Code review tool integration for posting PR comments
- **OpenAI API**: AI-powered code review via GPT-4o-mini model

## External Dependencies
- **curl**: For API calls and downloading reviewdog
- **jq**: JSON processing for API responses
- **git**: For diff generation and repository operations
- **reviewdog**: Automatically downloaded and installed during action execution

## File Structure
- `action.yml`: GitHub Action configuration
- `scripts/detect-language.sh`: Language detection logic
- `scripts/ai-review.sh`: AI review implementation
- `README.md`: Usage documentation
- `REQUIREMENT.md`: Detailed Vietnamese requirements specification

## Environment Requirements
- Runs on `ubuntu-latest`
- Requires `GITHUB_TOKEN` for PR comments
- Requires `OPENAI_API_KEY` for AI review functionality