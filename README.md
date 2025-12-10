# ssh-agent-mux

SSH Agent Multiplexer - A multiplexing SSH agent that stores keys locally and falls back to another agent (like 1Password).

## Overview

`ssh-agent-mux` is an SSH agent that acts as a multiplexer, allowing you to:

- **Add ephemeral keys locally** - Store temporary SSH keys that you need for short-term use
- **Fallback to another agent** - Seamlessly use keys from another SSH agent (like 1Password) when local keys don't match
- **Work with read-only agents** - Solves the problem where agents like 1Password don't accept key additions

This is particularly useful when you need to add ephemeral keys to your agent but still want to use 1Password (or another SSH agent) for normal operation.

## How It Works

When an SSH client makes a request:

1. **List Keys**: Returns all local keys first, then keys from the fallback agent
2. **Sign Request**: Tries to sign with local keys first, falls back to the fallback agent if the key isn't found locally
3. **Add Key**: Stores the key locally (fallback agents like 1Password typically don't support this)
4. **Remove Key**: Removes keys from local storage only

## Installation

### From Source

```bash
go install github.com/na4ma4/ssh-agent-mux/cmd/ssh-agent-mux@latest
```

### Build Locally

```bash
git clone https://github.com/na4ma4/ssh-agent-mux.git
cd ssh-agent-mux
go build ./cmd/ssh-agent-mux
```

## Usage

### Basic Usage

Start the agent with default settings (uses `$SSH_AUTH_SOCK` as fallback):

```bash
./ssh-agent-mux
```

The agent will print the socket path. Set it in your environment:

```bash
export SSH_AUTH_SOCK=/tmp/ssh-agent-mux-1000.sock
```

### With Custom Socket Path

```bash
./ssh-agent-mux -socket /path/to/custom.sock
```

### With Specific Fallback Agent

```bash
./ssh-agent-mux -fallback /path/to/1password/agent.sock
```

### With Verbose Logging

```bash
./ssh-agent-mux -verbose
```

### Complete Example with 1Password

```bash
# Start the multiplexer, using 1Password as fallback
./ssh-agent-mux -fallback ~/Library/Group\ Containers/2BUA8C4S2C.com.1password/t/agent.sock -verbose

# In another terminal, set the environment variable
export SSH_AUTH_SOCK=/tmp/ssh-agent-mux-1000.sock

# Add an ephemeral key
ssh-add ~/.ssh/temporary_key

# List all keys (local + 1Password)
ssh-add -l

# Use SSH as normal - local keys and 1Password keys work seamlessly
ssh user@example.com
```

## Command-Line Options

- `-socket <path>` - Path to Unix socket for the agent (default: `/tmp/ssh-agent-mux-<uid>.sock`)
- `-fallback <path>` - Path to fallback SSH agent socket (default: value of `$SSH_AUTH_SOCK`)
- `-verbose` - Enable verbose logging

## Use Cases

### 1Password + Ephemeral Keys

1Password's SSH agent doesn't allow adding keys via `ssh-add`. This multiplexer lets you:
- Use all your 1Password SSH keys normally
- Add temporary keys (build servers, temporary access, etc.) that you don't want to store in 1Password
- Have everything work through a single `SSH_AUTH_SOCK`

### Multiple Agents

You can chain multiple agents or use specialized agents for different purposes while presenting a unified interface to SSH clients.

## Architecture

```
┌─────────────────┐
│   SSH Client    │
└────────┬────────┘
         │
    ┌────▼────┐
    │ ssh-    │
    │ agent-  │  ┌──────────────┐
    │ mux     ├─►│ Local Keys   │
    │         │  │ (ephemeral)  │
    └────┬────┘  └──────────────┘
         │
         │ fallback
         │
    ┌────▼────────┐
    │  1Password  │
    │  SSH Agent  │
    └─────────────┘
```

## Technical Details

- Written in Go
- Uses `golang.org/x/crypto/ssh/agent` for SSH agent protocol
- Thread-safe key storage with mutex protection
- Implements the full SSH agent protocol interface

## License

See [LICENSE](LICENSE) file for details.

## Contributing

Contributions are welcome! Please feel free to submit issues or pull requests.

