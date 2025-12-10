.PHONY: build clean test install fmt vet

# Build the binary
build:
	go build -o ssh-agent-mux ./cmd/ssh-agent-mux

# Clean build artifacts
clean:
	rm -f ssh-agent-mux
	go clean

# Run tests
test:
	go test -v ./...

# Install the binary
install:
	go install ./cmd/ssh-agent-mux

# Format code
fmt:
	go fmt ./...

# Run go vet
vet:
	go vet ./...

# Run all checks
check: fmt vet test

# Build for multiple platforms
build-all:
	GOOS=linux GOARCH=amd64 go build -o ssh-agent-mux-linux-amd64 ./cmd/ssh-agent-mux
	GOOS=linux GOARCH=arm64 go build -o ssh-agent-mux-linux-arm64 ./cmd/ssh-agent-mux
	GOOS=darwin GOARCH=amd64 go build -o ssh-agent-mux-darwin-amd64 ./cmd/ssh-agent-mux
	GOOS=darwin GOARCH=arm64 go build -o ssh-agent-mux-darwin-arm64 ./cmd/ssh-agent-mux

# Help
help:
	@echo "Available targets:"
	@echo "  build      - Build the binary"
	@echo "  clean      - Clean build artifacts"
	@echo "  test       - Run tests"
	@echo "  install    - Install the binary"
	@echo "  fmt        - Format code"
	@echo "  vet        - Run go vet"
	@echo "  check      - Run fmt, vet, and test"
	@echo "  build-all  - Build for multiple platforms"
	@echo "  help       - Show this help message"
