package process

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// Info contains only process identity and terminal coordinates, never argv.
type Info struct {
	PID   int
	PPID  int
	TTY   string
	Start string
	Comm  string
}

func Inspect(ctx context.Context, pid int) (Info, error) {
	if pid <= 0 {
		return Info{}, fmt.Errorf("invalid PID %d", pid)
	}
	data, err := exec.CommandContext(ctx, "ps", "-p", strconv.Itoa(pid), "-o", "ppid=,tty=,lstart=,comm=").Output()
	if err != nil {
		return Info{}, fmt.Errorf("inspect PID %d: %w", pid, err)
	}
	return Parse(pid, string(data))
}

func Parse(pid int, data string) (Info, error) {
	fields := strings.Fields(data)
	if len(fields) < 8 {
		return Info{}, fmt.Errorf("incomplete ps output for PID %d", pid)
	}
	ppid, err := strconv.Atoi(fields[0])
	if err != nil {
		return Info{}, fmt.Errorf("invalid parent PID: %w", err)
	}
	return Info{
		PID: pid, PPID: ppid, TTY: NormalizeTTY(fields[1]),
		Start: strings.Join(fields[2:7], " "), Comm: strings.Join(fields[7:], " "),
	}, nil
}

func NormalizeTTY(tty string) string {
	if tty == "" || tty == "??" || tty == "?" || tty == "-" {
		return ""
	}
	if strings.HasPrefix(tty, "/dev/") {
		return tty
	}
	return "/dev/" + tty
}

func IsCodex(info Info) bool {
	return filepath.Base(info.Comm) == "codex"
}

func IsAgent(info Info, provider string) bool {
	switch provider {
	case "", "codex":
		return IsCodex(info)
	case "claude":
		return filepath.Base(info.Comm) == "claude"
	default:
		return false
	}
}

// CodexAncestor finds the CLI that launched the hook. It walks parents rather
// than relying on pane_current_command, which may report only the shell.
func CodexAncestor(ctx context.Context) (Info, error) {
	return AgentAncestor(ctx, "codex")
}

// AgentAncestor finds the provider CLI that launched a hook. GUI-hosted hooks
// legitimately have no matching CLI ancestor; callers can still index those
// sessions without registering a terminal coordinate.
func AgentAncestor(ctx context.Context, provider string) (Info, error) {
	pid := os.Getppid()
	for depth := 0; depth < 8 && pid > 1; depth++ {
		info, err := Inspect(ctx, pid)
		if err != nil {
			return Info{}, err
		}
		if IsAgent(info, provider) {
			return info, nil
		}
		pid = info.PPID
	}
	return Info{}, fmt.Errorf("%s CLI ancestor not found", provider)
}

// SameIdentity protects against PID reuse and against an unrelated process
// occupying a formerly registered pane.
func SameIdentity(ctx context.Context, pid int, start string) (Info, bool, error) {
	return SameIdentityFor(ctx, pid, start, "codex")
}

func SameIdentityFor(ctx context.Context, pid int, start, provider string) (Info, bool, error) {
	info, err := Inspect(ctx, pid)
	if err != nil {
		if probeErr := syscall.Kill(pid, 0); probeErr == syscall.ESRCH {
			return Info{}, false, nil
		}
		return Info{}, false, err // An inspection failure is not proof of exit.
	}
	return info, IsAgent(info, provider) && info.Start == start, nil
}

// CodexAtTTY is a conservative compatibility check for leases created before
// AgentPID was recorded. It does not claim which Codex thread owns that TTY.
func CodexAtTTY(ctx context.Context, tty string) (bool, error) {
	return AgentAtTTY(ctx, tty, "codex")
}

func AgentAtTTY(ctx context.Context, tty, provider string) (bool, error) {
	_, found, err := AgentInfoAtTTY(ctx, tty, provider)
	return found, err
}

// AgentInfoAtTTY returns the identity of a provider CLI attached to tty. It is
// used to recover a live terminal coordinate when a provider hook is executed
// by a detached daemon and therefore has no useful terminal ancestor.
func AgentInfoAtTTY(ctx context.Context, tty, provider string) (Info, bool, error) {
	tty = NormalizeTTY(tty)
	if tty == "" {
		return Info{}, false, nil
	}
	data, err := exec.CommandContext(ctx, "ps", "-t", strings.TrimPrefix(tty, "/dev/"), "-o", "pid=,ppid=,tty=,lstart=,comm=").Output()
	if err != nil {
		return Info{}, false, fmt.Errorf("inspect TTY %s: %w", tty, err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 9 {
			continue
		}
		pid, err := strconv.Atoi(fields[0])
		if err != nil {
			continue
		}
		info, err := Parse(pid, strings.Join(fields[1:], " "))
		if err == nil && IsAgent(info, provider) {
			return info, true, nil
		}
	}
	return Info{}, false, nil
}
