# Smart Code Review Action

This GitHub Action automatically detects the language/framework of your project and runs appropriate code review tools with [reviewdog](https://github.com/reviewdog/reviewdog).

## Supported Languages & Tools
- Node.js / TypeScript → ESLint
- Python → Flake8, Bandit
- Java → Maven Checkstyle
- Go → govet, golint
- .NET → dotnet format

## Usage

```yaml
name: Code Review

on:
  pull_request:

jobs:
  review:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: org/smart-code-review@v1
        with:
          github_token: ${{ secrets.GITHUB_TOKEN }}
```
