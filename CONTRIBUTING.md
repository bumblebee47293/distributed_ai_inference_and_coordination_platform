# Contributing to Distributed AI Inference Platform

Thank you for your interest in contributing! This document provides guidelines and instructions for contributing to this project.

## Code of Conduct

- Be respectful and inclusive
- Focus on constructive feedback
- Help others learn and grow

## Getting Started

1. **Fork the repository**
2. **Clone your fork:**
   ```bash
   git clone https://github.com/yourusername/distributed-ai-platform.git
   cd distributed-ai-platform
   ```
3. **Set up development environment:**
   ```bash
   ./scripts/setup/setup.sh
   ```

## Development Workflow

### 1. Create a Branch

```bash
git checkout -b feature/your-feature-name
```

Branch naming conventions:

- `feature/` - New features
- `fix/` - Bug fixes
- `docs/` - Documentation updates
- `refactor/` - Code refactoring
- `test/` - Test additions/updates

### 2. Make Changes

- Write clean, idiomatic Go code
- Follow the existing code style
- Add tests for new functionality
- Update documentation as needed

### 3. Test Your Changes

```bash
# Run unit tests
make test

# Run integration tests
make test-integration

# Run linter
make lint

# Test locally with Docker Compose
docker-compose up -d
```

### 4. Commit Your Changes

Follow conventional commits:

```bash
git commit -m "feat: add canary deployment support"
git commit -m "fix: resolve circuit breaker timeout issue"
git commit -m "docs: update API documentation"
```

### 5. Push and Create PR

```bash
git push origin feature/your-feature-name
```

Then create a Pull Request on GitHub.

## Code Style

### Go

- Follow [Effective Go](https://golang.org/doc/effective_go.html)
- Use `gofmt` for formatting
- Run `golangci-lint` before committing
- Write meaningful comments for exported functions
- Keep functions small and focused

### Example:

```go
// ProcessInference handles inference requests with retry logic.
// It returns the prediction result or an error if all retries fail.
func ProcessInference(ctx context.Context, req *InferenceRequest) (*InferenceResponse, error) {
    // Implementation
}
```

## Testing

### Unit Tests

- Test file naming: `*_test.go`
- Use table-driven tests where appropriate
- Aim for >80% code coverage
- Mock external dependencies

Example:

```go
func TestRouteInference(t *testing.T) {
    tests := []struct {
        name    string
        input   *RouteRequest
        want    *RouteResponse
        wantErr bool
    }{
        // Test cases
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Test implementation
        })
    }
}
```

### Integration Tests

- Located in `tests/integration/`
- Test service interactions
- Use Docker Compose for dependencies

## Documentation

- Update README.md for user-facing changes
- Add inline comments for complex logic
- Update API documentation
- Include examples where helpful

## Pull Request Guidelines

### PR Title

Use conventional commit format:

- `feat: description`
- `fix: description`
- `docs: description`
- `refactor: description`
- `test: description`

### PR Description

Include:

- **What:** Brief description of changes
- **Why:** Motivation and context
- **How:** Implementation approach
- **Testing:** How you tested the changes
- **Screenshots:** For UI changes (if applicable)

### PR Checklist

- [ ] Tests pass locally
- [ ] Code follows project style guidelines
- [ ] Documentation updated
- [ ] Commit messages follow conventions
- [ ] No merge conflicts
- [ ] PR is focused (one feature/fix per PR)

## Review Process

1. Automated checks must pass (CI/CD)
2. At least one maintainer approval required
3. Address review comments
4. Squash commits before merge (if requested)

## Questions?

- Open an issue for bugs or feature requests
- Start a discussion for questions
- Join our community chat (if available)

## License

By contributing, you agree that your contributions will be licensed under the MIT License.

---

Thank you for contributing! 🎉
