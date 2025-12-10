# Contributing to ssh-agent-mux

Thank you for your interest in contributing to ssh-agent-mux!

## Getting Started

1. Fork the repository
2. Clone your fork: `git clone https://github.com/YOUR_USERNAME/ssh-agent-mux.git`
3. Create a feature branch: `git checkout -b feature/your-feature-name`
4. Make your changes
5. Run tests: `make test`
6. Submit a pull request

## Development Setup

### Prerequisites

- Go 1.21 or later
- Make (optional, but recommended)

### Building

```bash
make build
# or
go build ./cmd/ssh-agent-mux
```

### Testing

```bash
make test
# or
go test ./...
```

### Code Formatting

Before committing, format your code:

```bash
make fmt
make vet
```

## Code Style

- Follow standard Go conventions
- Use `gofmt` to format code
- Write descriptive commit messages
- Add tests for new features
- Update documentation as needed

## Security

- Always use `crypto/rand.Reader` for cryptographic operations, never `nil`
- Test security-sensitive changes thoroughly
- Report security vulnerabilities privately to the maintainers

## Testing

- Write unit tests for new features
- Ensure all tests pass before submitting PR
- Aim for good test coverage

## Pull Request Process

1. Update the README.md with details of changes if needed
2. Update tests to reflect changes
3. Ensure the CI pipeline passes
4. Request review from maintainers

## Questions?

Feel free to open an issue for any questions or concerns.
