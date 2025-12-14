# ssh-agent-mux

**SSH Agent Multiplexer** - A long-running SSH agent that combines local ephemeral keys with backend agents like 1Password.

[![Go Version](https://img.shields.io/github/go-mod/go-version/na4ma4/ssh-agent-mux)](go.mod)
[![License](https://img.shields.io/github/license/na4ma4/ssh-agent-mux)](LICENSE)

## What Problem Does This Solve?

Modern SSH agents like **1Password** and **Secretive** are excellent for managing your permanent SSH keys securely, but they have one limitation: **you can't add temporary keys using `ssh-add`**. This becomes problematic when you need:

- Temporary deploy keys for CI/CD
- Short-lived keys for bastion/jump hosts
- Ephemeral keys for testing
- Keys you don't want to permanently store

`ssh-agent-mux` solves this by creating a multiplexing agent that:
1. Stores temporary keys locally in memory
2. Forwards all other requests to your backend agent (like 1Password)
3. Runs as a background daemon, just like a regular SSH agent

## How It Works

```
┌──────────────┐
│ SSH Client   │
│ (ssh, git)   │
└──────┬───────┘
       │
       │ SSH_AUTH_SOCK
       │
┌──────▼───────────────┐
│  ssh-agent-mux       │
│                      │
│  ┌────────────────┐  │
│  │ Local Keys     │  │ ← Keys added with ssh-add
│  │ (in memory)    │  │
│  └────────────────┘  │
│          │           │
│          ▼           │
│  ┌────────────────┐  │
│  │ Backend Agent  │  │ ← 1Password, Secretive, etc.
│  │ (fallback)     │  │
│  └────────────────┘  │
└──────────────────────┘
```

**Request Flow:**
- `ssh-add <key>` → Stores key **locally** (backend agents typically reject this)
- `ssh-add -l` → Lists **local keys** first, then **backend keys**
- `ssh user@host` → Tries **local keys** first, then **backend keys**
- `ssh-add -d <key>` → Removes from **local storage** only

## Installation

### Using Go Install

```bash
go install github.com/na4ma4/ssh-agent-mux/cmd/ssh-agent-mux@latest
```

### From Source

```bash
git clone https://github.com/na4ma4/ssh-agent-mux.git
cd ssh-agent-mux
mage build
# Binary will be in artifacts/build/release/
```

### Pre-built Binaries

Check the [Releases](https://github.com/na4ma4/ssh-agent-mux/releases) page for pre-built binaries.

## Quick Start

### 1. Start the Agent

The agent runs as a background daemon by default:

```bash
ssh-agent-mux
```

On first run, it will print the configuration:

```
SSH_AUTH_SOCK=/Users/yourname/.ssh/ssh-agent-mux.sock
SSH_AGENT_PID=12345
```

### 2. Configure Your Shell

Add to your `~/.zshrc` or `~/.bashrc`:

```bash
export SSH_AUTH_SOCK="$HOME/.ssh/ssh-agent-mux.sock"
```

### 3. Verify It's Working

```bash
# Check status
ssh-agent-mux -c config

# List keys (shows both local and backend keys)
ssh-add -l
```

## Usage Examples

### Adding a Temporary Key

```bash
# Generate a temporary key
ssh-keygen -t ed25519 -f ~/.ssh/temp_deploy_key -N ""

# Add it (stored locally in ssh-agent-mux)
ssh-add ~/.ssh/temp_deploy_key

# Use it
git clone git@github.com:yourorg/repo.git

# Remove when done
ssh-add -d ~/.ssh/temp_deploy_key
```

### Running with 1Password

```bash
# macOS with 1Password
ssh-agent-mux --backend-agent ~/Library/Group\ Containers/2BUA8C4S2C.com.1password/t/agent.sock

# Your 1Password keys will be available automatically
# Plus you can now add temporary keys with ssh-add
```

### Debug Mode

```bash
# Run in foreground with debug logging
ssh-agent-mux --foreground --debug

# Or send logs to a file
ssh-agent-mux --log-path /tmp/ssh-agent-mux.log
```

### Multiple Backend Agents

```bash
# Chain multiple backend agents
ssh-agent-mux \
  --backend-agent ~/Library/Group\ Containers/2BUA8C4S2C.com.1password/t/agent.sock \
  --backend-agent ~/.ssh/another-agent.sock
```

## Command-Line Options

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--socket` | `-s` | Path to Unix socket for the agent | `~/.ssh/ssh-agent-mux.sock` |
| `--backend-agent` | `-p` | Path to backend SSH agent socket (repeatable) | `$SSH_AUTH_SOCK` |
| `--foreground` | `-f` | Run in foreground (don't daemonize) | `false` |
| `--debug` | `-d` | Enable debug logging | `false` |
| `--quiet` | `-q` | Quiet output | `false` |
| `--log-path` | `-l` | Path to log file | stderr |
| `--command` | `-c` | Run a command (e.g., `config`, `ping`, `shutdown`) | - |
| `--help` | `-h` | Show help | - |
| `--version` | `-v` | Show version | - |

## Management Commands

### Check if Running

```bash
ssh-agent-mux -c config
```

Output:
```
Socket Path: /Users/yourname/.ssh/ssh-agent-mux.sock
Backend Socket Paths:
 - /Users/yourname/.config/1Password/agent.sock
PID: 12345
Start Time: 2025-12-14 10:30:00
Version: v1.0.0
```

### Ping the Agent

```bash
ssh-agent-mux -c ping
```

### Shutdown the Agent

```bash
ssh-agent-mux -c shutdown
```

## Common Use Cases

### 1. 1Password + Deploy Keys

**Problem:** You use 1Password for your personal SSH keys, but need to add deploy keys for CI/CD.

**Solution:**
```bash
# Start ssh-agent-mux pointing to 1Password
ssh-agent-mux --backend-agent ~/Library/Group\ Containers/2BUA8C4S2C.com.1password/t/agent.sock

# Add your deploy key temporarily
ssh-add ~/.ssh/deploy_key

# Both your 1Password keys and deploy key work
```

### 2. Temporary Access Keys

**Problem:** You need a temporary key for accessing a bastion host, but don't want to store it permanently.

**Solution:**
```bash
# Add the temporary key
ssh-add /tmp/bastion_key

# Use it
ssh -J bastion.example.com internal.example.com

# It's automatically removed when you remove it or restart the agent
ssh-add -d /tmp/bastion_key
```

### 3. Testing SSH Keys

**Problem:** You want to test a new SSH key without affecting your main agent configuration.

**Solution:**
```bash
# Start in foreground for testing
ssh-agent-mux --foreground --debug

# In another terminal
ssh-add ~/.ssh/test_key
ssh -T git@github.com
```

## Environment Variables

| Variable | Description |
|----------|-------------|
| `SSH_AGENT_MUX_SOCKET` | Override socket path |
| `SSH_AGENT_MUX_FOREGROUND` | Run in foreground if set to `1` or `true` |
| `SSH_AGENT_MUX_LOGPATH` | Log file path |
| `SSH_AUTH_SOCK` | Used as default backend agent path |
| `DEBUG` | Enable debug logging if set to `1` or `true` |

## Architecture & Technical Details

### Key Features

- **Thread-safe:** All operations are protected by mutexes
- **Memory-safe:** Fixed goroutine leaks and proper resource cleanup
- **Long-running:** Designed to run as a background daemon indefinitely
- **Graceful shutdown:** Properly handles SIGTERM/SIGINT signals
- **Zero configuration:** Works out of the box with sensible defaults

### Implementation

- **Language:** Go 1.24+
- **SSH Protocol:** `golang.org/x/crypto/ssh/agent`
- **Key Storage:** In-memory map with read-write mutex
- **Backend Communication:** Unix domain sockets
- **Process Management:** Daemon mode with proper signal handling

### Security Considerations

- Keys are stored **in memory only** (never written to disk)
- Socket permissions are restricted to user-only (0600)
- Backend agent credentials are never cached or stored
- All cryptographic operations use Go's standard crypto library

## Troubleshooting

### Agent Not Starting

```bash
# Check if socket already exists
ls -la ~/.ssh/ssh-agent-mux.sock

# Remove stale socket
rm ~/.ssh/ssh-agent-mux.sock

# Start in foreground to see errors
ssh-agent-mux --foreground --debug
```

### Keys Not Showing Up

```bash
# Verify the agent is running
ssh-agent-mux -c config

# Check SSH_AUTH_SOCK points to the right socket
echo $SSH_AUTH_SOCK

# List keys with verbose output
SSH_AUTH_SOCK=~/.ssh/ssh-agent-mux.sock ssh-add -l
```

### Can't Add Keys

```bash
# Make sure you're using the mux socket
export SSH_AUTH_SOCK="$HOME/.ssh/ssh-agent-mux.sock"

# Try adding with verbose output
ssh-add -v ~/.ssh/your_key
```

## Development

See [CONTRIBUTING.md](CONTRIBUTING.md) for development setup and guidelines.

### Building

```bash
# Using mage
mage build

# Or directly with Go
go build -o ssh-agent-mux ./cmd/ssh-agent-mux
```

### Testing

```bash
# Run tests
go test ./...

# Run demo script
./demo.sh
```

## License

See [LICENSE](LICENSE) file for details.

## Contributing

Contributions are welcome! Please:

1. Fork the repository
2. Create a feature branch
3. Make your changes with tests
4. Submit a pull request

See [CONTRIBUTING.md](CONTRIBUTING.md) for more details.

## Acknowledgments

- Inspired by the need to use 1Password's SSH agent with temporary keys
- Uses the excellent `golang.org/x/crypto/ssh/agent` library
- Daemon code adapted from [go-daemon](https://github.com/jaw0/go-daemon)

