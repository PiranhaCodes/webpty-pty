# webpty-pty

A production-grade PTY (pseudo-terminal) backend service that provides terminal session management over a UNIX domain socket. Designed for systemd daemon deployment with automatic shell detection, output streaming, and comprehensive resource cleanup.

## Features

- **UNIX Domain Socket API** - Fast, local IPC communication
- **Automatic Shell Detection** - Detects available shell in order: `$SHELL`, `/bin/bash`, `/bin/zsh`, `/bin/sh`
- **Dual Output Streaming** - Outputs to both FIFO pipes (real-time) and log files (persistent)
- **Thread-Safe Session Management** - Concurrent session handling with automatic cleanup
- **Graceful Shutdown** - Proper resource cleanup on termination
- **Production Ready** - Systemd-ready daemon with comprehensive error handling

## Architecture

```
┌─────────────┐
│   Client    │
└──────┬──────┘
       │ JSON over UNIX socket
       ▼
┌─────────────────────────────────────┐
│   webpty-pty Server                 │
│   /run/webpty/pty.sock              │
├─────────────────────────────────────┤
│  Session Manager                    │
│  ├── spawn                          │
│  ├── write                          │
│  ├── resize                         │
│  ├── kill                           │
│  └── list                           │
└──────┬──────────────────────────────┘
       │
       ▼
┌─────────────────────────────────────┐
│   PTY Session                      │
│   ├── FIFO: /run/webpty/sessions/  │
│   │        <id>.out                 │
│   └── Log:  /var/log/webpty/       │
│            <id>.log                 │
└─────────────────────────────────────┘
```

## Installation

### Prerequisites

- Go 1.23 or later
- Linux/Unix system with systemd (for daemon deployment)
- Root/sudo access (for `/run` and `/var/log` directories)

### Build

```bash
git clone https://github.com/PiranhaCodes/webpty-pty.git
cd webpty-pty
go build ./cmd/webpty-pty
```

### Install as Systemd Service

1. Copy the binary to a system directory:

```bash
sudo cp webpty-pty /usr/local/bin/
```

2. Create systemd service file `/etc/systemd/system/webpty-pty.service`:

```ini
[Unit]
Description=WebPTY PTY Backend Service
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/webpty-pty --config /etc/webpty/config.yml
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

3. Enable and start the service:

```bash
sudo systemctl daemon-reload
sudo systemctl enable webpty-pty
sudo systemctl start webpty-pty
```

4. Check status:

```bash
sudo systemctl status webpty-pty
```

## Usage

### Running the Server

```bash
# Run directly (requires sudo for /run and /var/log)
sudo ./webpty-pty --config /etc/webpty/config.yml

# Or with custom socket path
sudo ./webpty-pty --socket /tmp/webpty.sock
```

### API Actions

The server exposes a JSON-based API over a UNIX domain socket. See [protocol documentation](pkg/protocol/protocol.md) for complete details.

#### Spawn a Session

```bash
echo '{"action":"spawn","data":{}}' | nc -U /run/webpty/pty.sock
# Response: {"ok":true,"data":{"id":"abc-123-def"}}
```

#### Write to Session

```bash
echo '{"action":"write","data":{"id":"abc-123-def","data":"echo hello\n"}}' | nc -U /run/webpty/pty.sock
```

#### Resize Terminal

```bash
echo '{"action":"resize","data":{"id":"abc-123-def","cols":120,"rows":40}}' | nc -U /run/webpty/pty.sock
```

#### List Sessions

```bash
echo '{"action":"list","data":{}}' | nc -U /run/webpty/pty.sock
```

#### Kill Session

```bash
echo '{"action":"kill","data":{"id":"abc-123-def"}}' | nc -U /run/webpty/pty.sock
```

### Reading Output

#### From FIFO (Real-time)

```bash
cat /run/webpty/sessions/<session-id>.out
```

#### From Log File

```bash
tail -f /var/log/webpty/<session-id>.log
```

### Test Client

A test client is included to demonstrate usage:

```bash
# Build test client
go build ./test/testclient.go

# Run (requires server to be running)
sudo ./testclient
```

## Directory Structure

```
webpty-pty/
├── cmd/
│   └── webpty-pty/
│       └── main.go          # Main entry point
├── internal/
│   ├── api/
│   │   ├── server.go         # UNIX socket server
│   │   └── messages.go       # Protocol message types
│   └── pty/
│       ├── manager.go        # Session manager
│       ├── session.go        # Session handling
│       ├── spawn.go          # PTY spawning
│       ├── autodetect.go     # Shell detection
│       └── cleanup.go        # Resource cleanup
├── pkg/
│   └── protocol/
│       └── protocol.md       # Protocol documentation
├── test/
│   └── testclient.go         # Test client
├── go.mod
├── go.sum
└── README.md
```

## File Locations

- **Socket**: `/run/webpty/pty.sock`
- **FIFO Pipes**: `/run/webpty/sessions/<id>.out`
- **Log Files**: `/var/log/webpty/<id>.log`
- **Config File**: `/etc/webpty/config.yml` (optional, defaults used if missing)

## Protocol

The service communicates using JSON messages over a UNIX domain socket. Each request has an `action` and `data` field, and responses include an `ok` boolean and optional `err` or `data` fields.

For complete protocol documentation, see [pkg/protocol/protocol.md](pkg/protocol/protocol.md).

## Shell Detection

The service automatically detects an available shell in the following order:

1. `$SHELL` environment variable
2. `/bin/bash`
3. `/bin/zsh`
4. `/bin/sh`

If none are found, the spawn action returns an error.

## Session Lifecycle

1. **Creation**: Client sends `spawn` action → Server creates PTY, FIFO, and log file
2. **Active**: Client can send `write` and `resize` actions
3. **Termination**: Session ends via `kill` action, process exit, or server shutdown
4. **Cleanup**: All resources (PTY, FIFO, log file, process) are automatically cleaned up

## Error Handling

The service includes comprehensive error handling:

- Invalid requests return descriptive error messages
- Session not found errors for invalid IDs
- Resource creation failures are properly reported
- Process cleanup ensures no zombie processes
- Graceful shutdown with resource cleanup

## Development

### Building

```bash
go build ./cmd/webpty-pty
```

### Running Tests

```bash
go test ./...
```

### Code Style

This project follows [Google's Go Style Guide](https://google.github.io/styleguide/go/) for comments and code structure.

## License

MIT

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## Author

PiranhaCodes
