# Task Completion Checklist

## Before Committing Changes
1. **Script Validation**: Test shell script syntax with `bash -n script.sh`
2. **Permissions Check**: Ensure scripts are executable with `chmod +x`
3. **Environment Testing**: Verify scripts work with required environment variables
4. **Action Validation**: Check `action.yml` syntax is valid

## Testing Requirements
- **Local Testing**: Run scripts manually with test data when possible
- **Integration Testing**: Test the full GitHub Action workflow in a test repository
- **API Testing**: Verify API integrations (OpenAI, reviewdog) work as expected

## Security Considerations
- Never commit API keys or secrets
- Ensure environment variables are properly referenced
- Validate external script downloads (reviewdog install)
- Review curl commands for security implications

## Documentation Updates
- Update README.md if adding new supported languages
- Update REQUIREMENT.md if changing core functionality
- Ensure action.yml inputs/outputs are documented

## No Automated Linting/Formatting
This project doesn't have automated linting or formatting tools configured. Manual review of shell scripts and YAML files is required.