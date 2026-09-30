package codex

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

// appServer owns one stdio transport. Callers handle operation-specific replies
// while sharing initialization, response limits, and process cleanup.
type appServer struct {
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	encoder *json.Encoder
	scanner *bufio.Scanner
	stderr  bytes.Buffer
	closed  bool
}

func startAppServer(ctx context.Context) (*appServer, error) {
	rpc := &appServer{cmd: exec.CommandContext(ctx, executable(), "app-server", "--listen", "stdio://")}
	stdin, err := rpc.cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	rpc.stdin = stdin
	stdout, err := rpc.cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return nil, err
	}
	rpc.cmd.Stderr = &rpc.stderr
	if err := rpc.cmd.Start(); err != nil {
		_ = stdin.Close()
		return nil, fmt.Errorf("start Codex app-server: %w", err)
	}
	rpc.encoder = json.NewEncoder(stdin)
	rpc.scanner = bufio.NewScanner(stdout)
	rpc.scanner.Buffer(make([]byte, 64*1024), 8*1024*1024)
	if err := rpc.encoder.Encode(map[string]any{
		"method": "initialize", "id": 1,
		"params": map[string]any{"clientInfo": map[string]string{"name": "sesswitch", "title": "Sesswitch", "version": "0.1.0"}},
	}); err != nil {
		rpc.close()
		return nil, err
	}
	return rpc, nil
}

func (rpc *appServer) close() {
	if rpc.closed {
		return
	}
	rpc.closed = true
	_ = rpc.stdin.Close()
	_ = rpc.cmd.Process.Kill()
	_ = rpc.cmd.Wait()
}

func (rpc *appServer) endMessage() string {
	// Wait drains the stderr writer before its buffer is read.
	rpc.close()
	message := strings.TrimSpace(rpc.stderr.String())
	if message == "" {
		message = "app-server closed without a response"
	}
	return message
}
