# Project Purpose

This is a **Smart Code Review GitHub Action** that automatically detects the programming language/framework of a project and runs appropriate code review tools with reviewdog.

## Key Features
- Language/framework detection (Node.js/TypeScript, Python, Java, Go, .NET)
- Integration with reviewdog for PR comments
- AI-powered code review using OpenAI API
- Composite GitHub Action architecture

## Supported Languages & Tools
- Node.js / TypeScript → ESLint
- Python → Flake8, Bandit
- Java → Maven Checkstyle
- Go → govet, golint
- .NET → dotnet format

## Current Implementation
The action uses two main scripts:
1. `detect-language.sh` - Detects project language based on config files
2. `ai-review.sh` - Performs AI-powered code review using OpenAI API and outputs to reviewdog

## Usage Context
This is a GitHub Action intended to be used in CI/CD pipelines for automated code review on pull requests.