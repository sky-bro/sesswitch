package open

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/sky-bro/sesswitch/internal/hosts"
	"github.com/sky-bro/sesswitch/internal/session"
	"github.com/sky-bro/sesswitch/internal/toolenv"
)

type Runner = hosts.Runner
type OutputRunner = hosts.OutputRunner
type Verifier = hosts.Verifier

var ErrStaleLocation = hosts.ErrStaleLocation

func CommandRunner(ctx context.Context, name string, args ...string) error {
	cmd, err := toolenv.Command(ctx, name, args...)
	if err != nil {
		return err
	}
	cmd.Env = commandEnvironment(name, cmd.Env, wezTermSocketExists)
	if name == "wezterm" && len(args) > 0 && args[0] == "start" {
		return launchWezTerm(cmd)
	}
	return cmd.Run()
}

// A new GUI can outlive the launcher request. Keep it out of the request's
// process group, cancellation watcher and stdout/stderr pipes.
func launchWezTerm(cmd *exec.Cmd) error {
	log, err := os.CreateTemp("", "sesswitch-launch-*")
	if err != nil {
		return err
	}
	defer log.Close()
	// The child keeps its descriptor; no log file remains after it exits.
	if err := os.Remove(log.Name()); err != nil {
		return err
	}
	cmd.Stdout, cmd.Stderr = log, log
	cmd.Cancel = nil
	cmd.WaitDelay = 0
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	// Report immediate startup errors, but never wait for the window to close.
	timer := time.NewTimer(250 * time.Millisecond)
	defer timer.Stop()
	select {
	case err := <-done:
		if err != nil {
			_, _ = log.Seek(0, io.SeekStart)
			data, _ := io.ReadAll(io.LimitReader(log, 4096))
			if message := strings.TrimSpace(string(data)); message != "" {
				return fmt.Errorf("start WezTerm: %w: %s", err, message)
			}
		}
		return err
	case <-timer.C:
		return nil
	}
}

func CommandOutputRunner(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd, err := toolenv.Command(ctx, name, args...)
	if err != nil {
		return nil, err
	}
	cmd.Env = commandEnvironment(name, cmd.Env, wezTermSocketExists)
	return cmd.Output()
}

type socketCheck func(string) bool

// commandEnvironment prevents a long-lived tmux server from pinning WezTerm
// CLI calls to a GUI socket that disappeared after WezTerm restarted. A valid
// inherited socket is kept so multiple WezTerm instances still route correctly.
func commandEnvironment(name string, environment []string, socketExists socketCheck) []string {
	if name != "wezterm" {
		return environment
	}
	const key = "WEZTERM_UNIX_SOCKET"
	prefix := key + "="
	socket := ""
	found := false
	for index := len(environment) - 1; index >= 0; index-- {
		if strings.HasPrefix(environment[index], prefix) {
			socket = strings.TrimPrefix(environment[index], prefix)
			found = true
			break
		}
	}
	if !found || (socket != "" && socketExists(socket)) {
		return environment
	}
	filtered := make([]string, 0, len(environment)-1)
	for _, entry := range environment {
		if !strings.HasPrefix(entry, prefix) {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

func wezTermSocketExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode()&os.ModeSocket != 0
}

func Session(ctx context.Context, item session.Session, target string, run Runner) error {
	return sessionWithRunners(toolenv.Context(ctx), item, target, run, CommandOutputRunner, hosts.VerifyProcess)
}

func sessionWithRunners(ctx context.Context, item session.Session, target string, run Runner, output OutputRunner, verify Verifier) error {
	if item.Location != nil && target == "auto" {
		if err := hosts.Focus(ctx, *item.Location, run, output, verify); err == nil {
			return nil
		} else if !errors.Is(err, ErrStaleLocation) && !errors.Is(err, hosts.ErrSessionNotDisplayed) {
			return fmt.Errorf("could not focus registered location (not resuming automatically): %w", err)
		}
	}
	opener, ok := providerOpeners[item.Provider]
	if !ok {
		return fmt.Errorf("unsupported provider %q", item.Provider)
	}
	return opener.Open(ctx, item, target, run)
}

type ProviderOpener interface {
	Open(context.Context, session.Session, string, Runner) error
}

var providerOpeners = map[string]ProviderOpener{
	"codex":  codexOpener{},
	"claude": claudeOpener{},
}

type codexOpener struct{}

func (codexOpener) Open(ctx context.Context, item session.Session, target string, run Runner) error {
	if target == "auto" && item.Host != "tmux" && item.Host != "wezterm" {
		switch sourceHost(item.Source) {
		case "Codex Desktop":
			return run(ctx, "open", "codex://threads/"+item.ID)
		case "Chrome":
			if runtime.GOOS == "darwin" && item.BrowserTabID != "" {
				if err := hosts.FocusChromeTab(ctx, item.BrowserTabID, item.BrowserURL, run); err == nil {
					return nil
				}
			}
			return run(ctx, "open", "-b", "com.google.Chrome")
		case "VS Code":
			return run(ctx, "open", "-b", "com.microsoft.VSCode")
		}
	}
	switch target {
	case "app":
		if runtime.GOOS != "darwin" {
			return fmt.Errorf("app target is currently supported only on macOS")
		}
		return run(ctx, "open", "codex://threads/"+item.ID)
	case "auto", "terminal":
		args := []string{"start"}
		if item.CWD != "" {
			args = append(args, "--cwd", item.CWD)
		}
		args = append(args, "--", "codex", "resume", item.ID)
		return run(ctx, "wezterm", args...)
	default:
		return fmt.Errorf("unknown target %q", target)
	}
}

type claudeOpener struct{}

func (claudeOpener) Open(ctx context.Context, item session.Session, target string, run Runner) error {
	if target == "app" {
		return fmt.Errorf("Claude app deep links are not available for hook-observed sessions")
	}
	if target != "auto" && target != "terminal" {
		return fmt.Errorf("unknown target %q", target)
	}
	args := []string{"start"}
	if item.CWD != "" {
		args = append(args, "--cwd", item.CWD)
	}
	args = append(args, "--", "claude", "--resume", item.ID)
	return run(ctx, "wezterm", args...)
}

func sourceHost(source string) string {
	source = strings.ToLower(source)
	switch {
	case strings.Contains(source, "desktop"):
		return "Codex Desktop"
	case strings.Contains(source, "chrome"):
		return "Chrome"
	case strings.Contains(source, "vscode"), strings.Contains(source, "vs code"):
		return "VS Code"
	default:
		return "Terminal"
	}
}

func activateWezTermPane(ctx context.Context, paneID string, run Runner) error {
	return hosts.ActivateWezTermPane(ctx, paneID, run)
}
