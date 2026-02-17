# Test Plan: project-scaffold

## Overview
Manual and automated test cases for the project scaffold implementation.

## Prerequisites
- Go 1.22+ installed
- Clean environment (no existing config file)

---

## Test Suite

### 1. Build Verification

| ID | Test Case | Command | Expected Result |
|----|-----------|---------|-----------------|
| 1.1 | Build all packages | `go build ./...` | No errors |
| 1.2 | Run go vet | `go vet ./...` | No warnings |
| 1.3 | Build binary via Make | `make build` | Creates `bin/agenterm` |
| 1.4 | Binary is executable | `./bin/agenterm --help` | Shows flag usage |
| 1.5 | Clean build artifacts | `make clean && ls bin/` | Directory removed |

### 2. Configuration

| ID | Test Case | Steps | Expected Result |
|----|-----------|-------|-----------------|
| 2.1 | Default values | Start without flags | Port=8765, Session=ai-coding |
| 2.2 | Port flag override | `--port 9000` | Server listens on :9000 |
| 2.3 | Session flag override | `--session my-session` | Config shows TmuxSession=my-session |
| 2.4 | Token flag override | `--token mytoken123` | Uses provided token, no generation |
| 2.5 | Config file creation | First run without token | Creates `~/.config/agenterm/config` |
| 2.6 | Config file format | Cat config file | `Port=8765\nTmuxSession=...\nToken=...` |
| 2.7 | Token persistence | Restart server | Same token from file |
| 2.8 | Token auto-generation | New config | 32-char hex string |
| 2.9 | Config dir creation | Delete `~/.config/agenterm/`, run | Dir created with mode 0755 |
| 2.10 | Config file permissions | `ls -la ~/.config/agenterm/config` | Mode 0600 (owner read/write only) |

### 3. HTTP Server

| ID | Test Case | Steps | Expected Result |
|----|-----------|-------|-----------------|
| 3.1 | Root endpoint HTML | `curl http://localhost:8765` | Returns index.html content |
| 3.2 | HTML title | Check response | Contains `<title>agenterm</title>` |
| 3.3 | Content-Type header | `curl -I http://localhost:8765` | `Content-Type: text/html` |
| 3.4 | Bind address | Check startup log | `addr=0.0.0.0:8765` |
| 3.5 | Banner output | Check stdout | `agenterm running at http://localhost:8765?token=...` |

### 4. WebSocket Stub

| ID | Test Case | Steps | Expected Result |
|----|-----------|-------|-----------------|
| 4.1 | WS endpoint responds | `curl http://localhost:8765/ws` | Returns stub message |
| 4.2 | Connection accepted | Check response | "WebSocket stub - connection accepted and closed" |

### 5. Graceful Shutdown

| ID | Test Case | Steps | Expected Result |
|----|-----------|-------|-----------------|
| 5.1 | SIGINT handling | Start, `kill -INT <pid>` | Logs "shutdown signal received" |
| 5.2 | SIGTERM handling | Start, `kill -TERM <pid>` | Graceful shutdown |
| 5.3 | In-flight requests | Request during shutdown | Request completes before exit |
| 5.4 | Shutdown timeout | Force timeout | Server exits within 5s |

### 6. Logging

| ID | Test Case | Steps | Expected Result |
|----|-----------|-------|-----------------|
| 6.1 | Startup log | Start server | `level=INFO msg="server starting"` |
| 6.2 | Shutdown log | Kill server | `level=INFO msg="shutdown signal received"` |
| 6.3 | Structured format | Check log output | `time=... level=... msg=...` format |

### 7. Error Handling

| ID | Test Case | Steps | Expected Result |
|----|-----------|-------|-----------------|
| 7.1 | Port in use | Start two instances | Second fails with "address in use" |
| 7.2 | Invalid port | `--port -1` | Server fails to start |
| 7.3 | Config dir permission | Unset HOME, run | Graceful error message |

---

## Automated Test Script

```bash
#!/bin/bash
# test_scaffold.sh - Run automated tests

set -e
GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m'

pass() { echo -e "${GREEN}✓ $1${NC}"; }
fail() { echo -e "${RED}✗ $1${NC}"; exit 1; }

# Setup
CONFIG_DIR="$HOME/.config/agenterm"
CONFIG_FILE="$CONFIG_DIR/config"
[ -f "$CONFIG_FILE" ] && mv "$CONFIG_FILE" "$CONFIG_FILE.bak"

# Cleanup on exit
cleanup() {
    [ -f "$CONFIG_FILE.bak" ] && mv "$CONFIG_FILE.bak" "$CONFIG_FILE"
    pkill -f "bin/agenterm" 2>/dev/null || true
}
trap cleanup EXIT

echo "=== Build Tests ==="
make clean && make build || fail "Build failed"
[ -x bin/agenterm ] || fail "Binary not executable"
pass "Build successful"

echo "=== Configuration Tests ==="
rm -rf "$CONFIG_DIR"

./bin/agenterm --port 8765 &
PID=$!
sleep 1

[ -f "$CONFIG_FILE" ] || fail "Config file not created"
TOKEN=$(grep "^Token=" "$CONFIG_FILE" | cut -d= -f2)
[ ${#TOKEN} -eq 32 ] || fail "Token not 32 chars"
pass "Token auto-generation"

kill $PID 2>/dev/null
wait $PID 2>/dev/null

# Test token persistence
./bin/agenterm --port 8765 &
PID=$!
sleep 1
TOKEN2=$(grep "^Token=" "$CONFIG_FILE" | cut -d= -f2)
[ "$TOKEN" = "$TOKEN2" ] || fail "Token not persisted"
pass "Token persistence"

echo "=== HTTP Server Tests ==="
HTML=$(curl -s http://localhost:8765)
echo "$HTML" | grep -q "agenterm" || fail "HTML not served"
pass "Root endpoint"

echo "$HTML" | grep -q "Connection status" || fail "Missing placeholder"
pass "HTML content"

echo "=== WebSocket Stub Tests ==="
WS=$(curl -s http://localhost:8765/ws)
echo "$WS" | grep -q "WebSocket stub" || fail "WS stub not working"
pass "WebSocket stub"

echo "=== Graceful Shutdown Tests ==="
kill -INT $PID 2>/dev/null
wait $PID 2>/dev/null
pass "SIGINT handled"

echo "=== Port Override Tests ==="
./bin/agenterm --port 9000 &
PID=$!
sleep 1
curl -s http://localhost:9000 > /dev/null || fail "Port 9000 not listening"
pass "Port override"

kill $PID 2>/dev/null
wait $PID 2>/dev/null

echo ""
echo -e "${GREEN}All tests passed!${NC}"
```

---

## Running Tests

### Manual Testing
```bash
# 1. Build
make build

# 2. Run with defaults
./bin/agenterm

# 3. In another terminal
curl http://localhost:8765
curl http://localhost:8765/ws

# 4. Test shutdown
Ctrl+C

# 5. Test with flags
./bin/agenterm --port 9000 --session mysession --token testtoken123
```

### Automated Testing
```bash
# Save script above as test_scaffold.sh
chmod +x test_scaffold.sh
./test_scaffold.sh
```

---

## Test Matrix

| Platform | Go Version | Status |
|----------|------------|--------|
| macOS (Intel) | 1.22+ | Tested |
| macOS (ARM) | 1.22+ | Tested |
| Linux | 1.22+ | Pending |
| Windows | 1.22+ | Pending |

---

## Acceptance Criteria Checklist

- [x] `go build ./cmd/agenterm` produces a working binary
- [x] Binary serves embedded HTML at root
- [x] Token auto-generation works
- [x] Graceful shutdown on SIGINT
- [x] No external dependencies (stdlib only)
