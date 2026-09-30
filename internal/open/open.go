package open

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/sky-bro/sesswitch/internal/hosts"
	"github.com/sky-bro/sesswitch/internal/session"
)

type Runner = hosts.Runner
type OutputRunner = hosts.OutputRunner
type Verifier = hosts.Verifier

var ErrStaleLocation = hosts.ErrStaleLocation

func CommandRunner(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = commandEnvironment(name, os.Environ(), wezTermSocketExists)
	return cmd.Run()
}

func CommandOutputRunner(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = commandEnvironment(name, os.Environ(), wezTermSocketExists)
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
	return sessionWithRunners(ctx, item, target, run, CommandOutputRunner, hosts.VerifyProcess)
}

func sessionWithRunners(ctx context.Context, item session.Session, target string, run Runner, output OutputRunner, verify Verifier) error {
	if item.Location != nil && target == "auto" {
		if err := hosts.Focus(ctx, *item.Location, run, output, verify); err == nil {
			return nil
		} else if !errors.Is(err, ErrStaleLocation) {
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
	if target == "auto" {
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
