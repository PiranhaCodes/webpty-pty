# WebPTY Protocol Specification

## Overview

The WebPTY backend service communicates over a UNIX domain socket using JSON messages. The socket is located at `~/.webpty/pty.sock` (expanded to the user's home directory).

## Message Format

All messages are JSON objects sent over the UNIX socket connection.

### Request Format

```json
{
  "action": "spawn" | "write" | "resize" | "kill" | "list",
  "data": { ... }
}
```

- `action`: The action to perform (required)
- `data`: Action-specific data (required, can be empty object `{}`)

### Response Format

```json
{
  "ok": true | false,
  "err": "error message (optional, only present if ok is false)",
  "data": { ... }
}
```

- `ok`: Boolean indicating success or failure
- `err`: Error message string (only present when `ok` is false)
- `data`: Response data (only present when `ok` is true and action returns data)

## Actions

### spawn

Creates a new PTY session with an auto-detected shell.

**Request:**

```json
{
  "action": "spawn",
  "data": {}
}
```

**Response (Success):**

```json
{
  "ok": true,
  "data": {
    "id": "session-uuid"
  }
}
```

**Response (Error):**

```json
{
  "ok": false,
  "err": "error message"
}
```

**Shell Detection Order:**

1. `$SHELL` environment variable
2. `/bin/bash`
3. `/bin/zsh`
4. `/bin/sh`

If none are found, an error is returned.

**Output Streams:**

- FIFO pipe: `~/.webpty/sessions/<id>.out`
- Log file: `~/.webpty/log/<id>.log`

### write

Sends data to the PTY stdin.

**Request:**

```json
{
  "action": "write",
  "data": {
    "id": "session-uuid",
    "data": "string to send"
  }
}
```

**Response (Success):**

```json
{
  "ok": true
}
```

**Response (Error):**

```json
{
  "ok": false,
  "err": "session not found"
}
```

### resize

Resizes the PTY terminal.

**Request:**

```json
{
  "action": "resize",
  "data": {
    "id": "session-uuid",
    "cols": 80,
    "rows": 24
  }
}
```

- `cols`: Number of columns (must be > 0)
- `rows`: Number of rows (must be > 0)

**Response (Success):**

```json
{
  "ok": true
}
```

**Response (Error):**

```json
{
  "ok": false,
  "err": "session not found"
}
```

### kill

Terminates a PTY session and cleans up all resources.

**Request:**

```json
{
  "action": "kill",
  "data": {
    "id": "session-uuid"
  }
}
```

**Response (Success):**

```json
{
  "ok": true
}
```

**Response (Error):**

```json
{
  "ok": false,
  "err": "session not found"
}
```

**Cleanup:**

- Closes PTY file descriptor
- Closes log file
- Closes FIFO writer
- Removes FIFO file
- Kills subprocess (SIGTERM, then SIGKILL if needed)
- Removes session from manager

### list

Returns all active PTY sessions.

**Request:**

```json
{
  "action": "list",
  "data": {}
}
```

**Response (Success):**

```json
{
  "ok": true,
  "data": {
    "sessions": [
      {
        "id": "session-uuid-1",
        "status": "active"
      },
      {
        "id": "session-uuid-2",
        "status": "exiting"
      }
    ],
    "count": 2
  }
}
```

**Status Values:**

- `active`: Session is running normally
- `exiting`: Session is in the process of shutting down

## Error Codes

Common error messages:

- `"invalid request"`: Malformed JSON or missing required fields
- `"unknown action"`: Action not recognized
- `"session not found"`: Session ID does not exist
- `"session ID is required"`: Missing ID in request data
- `"cols and rows must be positive"`: Invalid resize dimensions
- `"no shell found: ..."`: Shell detection failed
- `"failed to start PTY: ..."`: PTY creation failed
- `"failed to create FIFO: ..."`: FIFO creation failed
- `"failed to open log file: ..."`: Log file creation failed

## Session Lifecycle

1. **Creation**: Client sends `spawn` action

   - Server detects shell
   - Creates PTY
   - Creates FIFO pipe at `~/.webpty/sessions/<id>.out`
   - Opens log file at `~/.webpty/log/<id>.log`
   - Starts read loop in background
   - Returns session ID

2. **Active**: Client can send `write` and `resize` actions

   - All PTY output is written to both FIFO and log file
   - FIFO is opened in non-blocking mode for writing

3. **Termination**: Session ends when:

   - Client sends `kill` action
   - PTY process exits (detected in read loop)
   - Server shuts down

4. **Cleanup**: Automatic cleanup on termination
   - All file descriptors closed
   - FIFO file removed
   - Process killed if still running
   - Session removed from manager

## File Locations

- **Socket**: `~/.webpty/pty.sock` (expanded to user's home directory)
- **FIFO Pipes**: `~/.webpty/sessions/<id>.out`
- **Log Files**: `~/.webpty/log/<id>.log`
- **Config File**: `~/.webpty/config.yml` (optional, defaults used if missing)

## Concurrency

- The server handles multiple concurrent connections
- Each connection is handled in a separate goroutine
- Session manager uses read-write locks for thread safety
- FIFO writes are non-blocking to prevent deadlocks

## Example Session

```bash
# 1. Spawn session
echo '{"action":"spawn","data":{}}' | nc -U ~/.webpty/pty.sock
# Response: {"ok":true,"data":{"id":"abc-123-def"}}

# 2. Write command
echo '{"action":"write","data":{"id":"abc-123-def","data":"echo hello\n"}}' | nc -U ~/.webpty/pty.sock
# Response: {"ok":true}

# 3. Read output from FIFO
cat ~/.webpty/sessions/abc-123-def.out
# Output: hello

# 4. Resize terminal
echo '{"action":"resize","data":{"id":"abc-123-def","cols":120,"rows":40}}' | nc -U ~/.webpty/pty.sock
# Response: {"ok":true}

# 5. List sessions
echo '{"action":"list","data":{}}' | nc -U ~/.webpty/pty.sock
# Response: {"ok":true,"data":{"sessions":[{"id":"abc-123-def","status":"active"}],"count":1}}

# 6. Kill session
echo '{"action":"kill","data":{"id":"abc-123-def"}}' | nc -U ~/.webpty/pty.sock
# Response: {"ok":true}
```
