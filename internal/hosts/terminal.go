package hosts

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"runtime"
	"strconv"
	"strings"

	"github.com/sky-bro/sesswitch/internal/process"
	"github.com/sky-bro/sesswitch/internal/session"
)

type Runner func(context.Context, string, ...string) error
type OutputRunner func(context.Context, string, ...string) ([]byte, error)
type Verifier func(context.Context, session.Location, string) error

var ErrStaleLocation = errors.New("registered agent process is no longer running")

// Focus routes a verified terminal location to its concrete host adapter.
// tmux owns pane selection while WezTerm owns GUI-pane and app activation.
func Focus(ctx context.Context, location session.Location, run Runner, output OutputRunner, verify Verifier) error {
	if location.AgentPID > 0 && location.AgentStart != "" && location.TTY != "" {
		if err := verify(ctx, location, process.NormalizeTTY(location.TTY)); err != nil {
			return err
		}
	}
	if location.TmuxPane != "" {
		paneData, err := output(ctx, "tmux", "display-message", "-p", "-t", location.TmuxPane, "#{session_id}\t#{pane_tty}")
		if err != nil {
			return fmt.Errorf("locate tmux pane %s: %w", location.TmuxPane, err)
		}
		parts := strings.SplitN(strings.TrimSpace(string(paneData)), "\t", 2)
		if len(parts) != 2 || parts[0] == "" || process.NormalizeTTY(parts[1]) == "" {
			return fmt.Errorf("tmux pane %s has no usable session/TTY", location.TmuxPane)
		}
		sessionID, paneTTY := parts[0], process.NormalizeTTY(parts[1])
		if err := verify(ctx, location, paneTTY); err != nil {
			return err
		}

		panes, err := wezTermPanes(ctx, output)
		if err != nil {
			panes = nil
		}
		clients, err := tmuxClients(ctx, output)
		if err != nil {
			return err
		}
		if pane, ok := reusableTmuxClient(panes, clients, location.WezTermPane); ok {
			if err := run(ctx, "tmux", "switch-client", "-c", pane.TTYName, "-t", location.TmuxPane); err != nil {
				return err
			}
			return ActivateWezTermPane(ctx, strconv.FormatUint(pane.PaneID, 10), run)
		}

		if err := run(ctx, "tmux", "select-window", "-t", location.TmuxPane); err != nil {
			return err
		}
		if err := run(ctx, "tmux", "select-pane", "-t", location.TmuxPane); err != nil {
			return err
		}
		if pane, ok := activeWezTermPane(panes); ok {
			spawned, err := output(ctx, "wezterm", "cli", "spawn", "--window-id", strconv.FormatUint(pane.WindowID, 10), "--", "tmux", "attach-session", "-t", sessionID)
			if err != nil {
				return err
			}
			paneID := strings.TrimSpace(string(spawned))
			if _, err := strconv.ParseUint(paneID, 10, 64); err != nil {
				return fmt.Errorf("invalid spawned WezTerm pane id %q", paneID)
			}
			return ActivateWezTermPane(ctx, paneID, run)
		}
		return run(ctx, "wezterm", "start", "--", "tmux", "attach-session", "-t", sessionID)
	}
	if location.WezTermPane != "" {
		id, err := strconv.ParseUint(location.WezTermPane, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid WezTerm pane id")
		}
		panes, err := wezTermPanes(ctx, output)
		if err != nil {
			return err
		}
		for _, pane := range panes {
			if pane.PaneID == id {
				if err := verify(ctx, location, pane.TTYName); err != nil {
					return err
				}
				return ActivateWezTermPane(ctx, location.WezTermPane, run)
			}
		}
		return fmt.Errorf("WezTerm pane %s is not available", location.WezTermPane)
	}
	return fmt.Errorf("location has no focusable host")
}

func VerifyProcess(ctx context.Context, location session.Location, paneTTY string) error {
	if paneTTY == "" {
		return fmt.Errorf("pane TTY is unavailable")
	}
	if location.AgentPID > 0 && location.AgentStart != "" {
		info, same, err := process.SameIdentityFor(ctx, location.AgentPID, location.AgentStart, location.Provider)
		if err != nil {
			return err
		}
		if !same {
			return ErrStaleLocation
		}
		if info.TTY != paneTTY {
			return fmt.Errorf("agent PID %d moved to %s; recorded pane is %s", location.AgentPID, info.TTY, paneTTY)
		}
		return nil
	}
	found, err := process.AgentAtTTY(ctx, paneTTY, location.Provider)
	if err != nil {
		return err
	}
	if !found {
		return ErrStaleLocation
	}
	return nil
}

func ActivateWezTermPane(ctx context.Context, paneID string, run Runner) error {
	if err := run(ctx, "wezterm", "cli", "activate-pane", "--pane-id", paneID); err != nil {
		return err
	}
	if runtime.GOOS == "darwin" {
		return run(ctx, "open", "-a", "WezTerm")
	}
	return nil
}

type wezTermPane struct {
	WindowID uint64 `json:"window_id"`
	PaneID   uint64 `json:"pane_id"`
	TTYName  string `json:"tty_name"`
	IsActive bool   `json:"is_active"`
}

func wezTermPanes(ctx context.Context, output OutputRunner) ([]wezTermPane, error) {
	data, err := output(ctx, "wezterm", "cli", "list", "--format", "json")
	if err != nil {
		return nil, fmt.Errorf("list WezTerm panes: %w", err)
	}
	var panes []wezTermPane
	if err := json.Unmarshal(data, &panes); err != nil {
		return nil, fmt.Errorf("decode WezTerm panes: %w", err)
	}
	return panes, nil
}

func tmuxClients(ctx context.Context, output OutputRunner) (map[string]bool, error) {
	data, err := output(ctx, "tmux", "list-clients", "-F", "#{client_tty}")
	if err != nil {
		return map[string]bool{}, nil
	}
	clients := make(map[string]bool)
	for _, tty := range strings.Fields(string(data)) {
		clients[tty] = true
	}
	return clients, nil
}

func reusableTmuxClient(panes []wezTermPane, clients map[string]bool, preferredPaneID string) (wezTermPane, bool) {
	if preferredPaneID != "" {
		if id, err := strconv.ParseUint(preferredPaneID, 10, 64); err == nil {
			for _, pane := range panes {
				if pane.PaneID == id && clients[pane.TTYName] {
					return pane, true
				}
			}
		}
	}
	for _, pane := range panes {
		if pane.IsActive && clients[pane.TTYName] {
			return pane, true
		}
	}
	for _, pane := range panes {
		if clients[pane.TTYName] {
			return pane, true
		}
	}
	return wezTermPane{}, false
}

func activeWezTermPane(panes []wezTermPane) (wezTermPane, bool) {
	for _, pane := range panes {
		if pane.IsActive {
			return pane, true
		}
	}
	if len(panes) > 0 {
		return panes[0], true
	}
	return wezTermPane{}, false
}
