# Sample Project for Testing Smart Code Review Action

This project contains sample code with intentional code quality issues to test the Smart Code Review GitHub Action.

## Languages & Issues Included

### JavaScript/Node.js
- **Files**: `index.js`, `utils.js`, `package.json`
- **Issues**: Missing semicolons, var usage, assignment vs comparison, unused variables, magic numbers, debug statements, complex nested conditions

### Python
- **Files**: `app.py`, `requirements.txt`
- **Issues**: Poor formatting, missing spaces, security vulnerabilities (eval), missing type hints, hardcoded values, debug mode in production

### Go
- **Files**: `main.go`, `go.mod`
- **Issues**: Missing error handling, hardcoded values, inefficient algorithms, poor logging practices, missing input validation

## How to Test

1. **Push this sample project to a GitHub repository**
2. **Set up the Smart Code Review Action** in your repository
3. **Add required secrets**:
   - `OPENAI_API_KEY`: Your OpenAI API key
4. **Create a pull request** with changes to trigger the action
5. **Check PR comments** for AI-powered code review feedback

## Expected Behavior

The GitHub Action should:
1. Detect multiple languages (Node.js, Python, Go)
2. Run appropriate linters for each language
3. Provide AI-powered code review comments
4. Post feedback directly on the PR via reviewdog

## Manual Testing

You can also test the detection script locally:

```bash
cd sample-project
export GITHUB_TOKEN="your_token"
export OPENAI_API_KEY="your_openai_key"
bash ../scripts/detect-language.sh
```

This will detect the primary language and run the appropriate review tools.