# Contributing to AccessGraph

Thank you for your interest in contributing to AccessGraph! This document provides guidelines for contributing to the project.

## Getting Started

1. Fork the repository
2. Clone your fork: `git clone https://github.com/YOUR_USERNAME/accessgraph.git`
3. Add upstream remote: `git remote add upstream https://github.com/jamesolaitan/accessgraph.git`
4. Create a branch: `git checkout -b feature/your-feature-name`

## Development Setup

### Prerequisites

- Go 1.22 or later
- Node.js 18 or later
- Docker and Docker Compose
- golangci-lint
- gosec (optional)

### Build and Test

```bash
# Install dependencies
make deps

# Build binaries
make build

# Run tests
make test

# Run linter
make lint

# Run security scanner
make sec

# Build UI
make ui
```

## Code Standards

### Go Code

- Follow standard Go formatting (`gofmt`)
- Use meaningful variable and function names
- Add comments for exported functions and types
- Write tests for new functionality
- Maintain test coverage above 70%

### React/TypeScript

- Use functional components with hooks
- Follow TypeScript strict mode
- Use meaningful component and prop names
- Keep components focused and reusable

### Commit Messages

Use conventional commit format:

```
type(scope): description

[optional body]

[optional footer]
```

Types:
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `test`: Test changes
- `refactor`: Code refactoring
- `chore`: Build/tooling changes

Examples:
- `feat(ingest): add GCP IAM parser`
- `fix(api): correct snapshot diff edge comparison`
- `docs(readme): update installation instructions`

## Pull Request Process

1. Update documentation for any changed functionality
2. Add tests for new features
3. Ensure all tests pass: `make test`
4. Ensure linting passes: `make lint`
5. Update CHANGELOG.md with your changes
6. Submit PR with clear description of changes
7. Wait for review and address feedback

## Testing Requirements

- Unit tests for all new functions
- Integration tests for new features
- Maintain overall coverage ≥70%
- Test both success and error paths
- Include edge cases

## Security Considerations

- No secrets in code or logs
- Use redaction for sensitive data
- Respect offline mode requirements
- Test security features thoroughly
- Report vulnerabilities privately to maintainers

## Documentation

- Update README.md for user-facing changes
- Add inline code comments for complex logic
- Update API documentation for endpoint changes
- Include examples in documentation

## Questions?

Open an issue or reach out to maintainers.

## License

By contributing, you agree that your contributions will be licensed under the Apache License 2.0.

