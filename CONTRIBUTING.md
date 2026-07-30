# Contributing to Task Queue Educational Dashboard

First off, thank you for considering contributing! This project is educational by nature, so contributions that improve clarity, add learning value, or fix bugs are especially welcome.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [How Can I Contribute?](#how-can-i-contribute)
- [Getting Started](#getting-started)
- [Development Setup](#development-setup)
- [Pull Request Process](#pull-request-process)
- [Style Guidelines](#style-guidelines)

## Code of Conduct

This project adheres to the [Contributor Covenant Code of Conduct](CODE_OF_CONDUCT.md). By participating, you are expected to uphold this code.

## How Can I Contribute?

### Reporting Bugs

- Check existing issues first to avoid duplicates
- Use the [bug report template](.github/ISSUE_TEMPLATE/bug_report.md)
- Include steps to reproduce, expected vs actual behavior
- Include your environment (OS, Docker version, Go version)

### Suggesting Features

- Use the [feature request template](.github/ISSUE_TEMPLATE/feature_request.md)
- Explain the educational value of the feature
- Describe your use case

### Good First Issues

Look for issues labeled [`good first issue`](https://github.com/kripa-sindhu-007/task-queue-educational-dashboard/labels/good%20first%20issue) — these are beginner-friendly tasks perfect for your first contribution.

### Documentation

- Fix typos, improve explanations
- Add diagrams or annotations
- Improve inline code comments for educational clarity

### Code Contributions

- Bug fixes
- New task handler types
- Dashboard improvements
- Test coverage
- Performance improvements with educational annotations

## Getting Started

1. Fork the repository
2. Clone your fork:
   ```bash
   git clone https://github.com/<your-username>/task-queue-educational-dashboard.git
   cd task-queue-educational-dashboard
   ```
3. Create a branch for your work:
   ```bash
   git checkout -b feature/your-feature-name
   ```

## Development Setup

### Prerequisites

- Go 1.22+
- Node.js 20+
- Docker & Docker Compose
- Redis 7+ (or use Docker)

### Running Locally

**With Docker (recommended):**
```bash
docker compose up --build
```

**Without Docker:**

Backend:
```bash
cd backend
go run ./cmd/server
# In another terminal:
go run ./cmd/worker
```

Frontend:
```bash
cd frontend
npm install
npm run dev
```

### Running Tests

Backend:
```bash
cd backend
go test ./...
```

Frontend:
```bash
cd frontend
npm run lint
```

## Pull Request Process

1. Ensure your code passes all tests and linting
2. Update documentation if you're changing behavior
3. Add comments explaining *why* for non-obvious code (this is an educational project!)
4. Fill out the PR template completely
5. Keep PRs focused — one feature or fix per PR
6. Rebase on `main` before submitting

### Commit Messages

Use clear, descriptive commit messages:

```
feat: add rate limiting to task submission endpoint
fix: correct retry backoff calculation for edge case
docs: add sequence diagram for worker lifecycle
test: add integration test for dead-letter flow
```

Prefixes: `feat`, `fix`, `docs`, `test`, `refactor`, `chore`, `style`

## Style Guidelines

### Go Code

- Follow standard `gofmt` formatting
- Use meaningful variable names (avoid single letters except in loops)
- Add comments explaining the *educational purpose* of complex logic
- Keep functions focused and under 50 lines where possible
- Use table-driven tests

### TypeScript/React Code

- Use TypeScript strict mode
- Prefer named exports
- Use functional components with hooks
- Follow the existing Tailwind CSS patterns

### Lua Scripts

- Add header comments explaining what the script does atomically
- Comment each Redis command with its purpose

## Questions?

Open a [discussion](https://github.com/kripa-sindhu-007/task-queue-educational-dashboard/issues) or reach out via issues. No question is too basic — this is a learning project!
