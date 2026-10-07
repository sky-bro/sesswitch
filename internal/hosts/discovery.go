package hosts

import (
	"context"
	"path/filepath"
	"strings"
	"time"

	"github.com/sky-bro/sesswitch/internal/process"
	"github.com/sky-bro/sesswitch/internal/session"
)

type AgentAtTTY func(context.Context, string, string) (process.Info, bool, error)

type tmuxPane struct {
	id    string
	tty   string
	cwd   string
	title string
}

// DiscoverTmuxLocations conservatively recovers locations missed by hooks and
// refreshes registered tmux locations with live host state. A session is linked
// only when exactly one pane has the same cwd and title and contains a live
// provider CLI. A registered pane is refreshed by its pane and process identity
// so provider-side renames cannot freeze stale title-derived state.
func DiscoverTmuxLocations(ctx context.Context, items []session.Session, output OutputRunner, agentAtTTY AgentAtTTY) map[string]session.Location {
	result := make(map[string]session.Location)
	data, err := output(ctx, "tmux", "list-panes", "-a", "-F", "#{pane_id}\t#{pane_tty}\t#{pane_current_path}\t#{pane_title}")
	if err != nil {
		return result
	}
	panes := parseTmuxPanes(string(data))
	claims := make(map[string][]session.Session)
	for _, item := range items {
		if item.Key == "" {
			continue
		}
		if item.Location != nil && item.Location.TmuxPane == "" {
			continue
		}
		var matches []session.Location
		for _, pane := range panes {
			registered := item.Location != nil && item.Location.TmuxPane != ""
			if registered {
				if item.Location.TmuxPane != pane.id {
					continue
				}
			} else if item.Title == "" || item.CWD == "" || !samePath(item.CWD, pane.cwd) || !paneTitleMatches(conversationTitle(pane), item.Title) {
				continue
			}
			info, found, err := agentAtTTY(ctx, pane.tty, item.Provider)
			if err != nil || !found || info.TTY != process.NormalizeTTY(pane.tty) {
				continue
			}
			if registered && item.Location.AgentPID > 0 && item.Location.AgentStart != "" &&
				(info.PID != item.Location.AgentPID || info.Start != item.Location.AgentStart) {
				continue
			}
			location := session.Location{
				Provider: item.Provider, Kind: "tmux", TmuxPane: pane.id,
				TTY: info.TTY, AgentPID: info.PID, AgentStart: info.Start,
				NeedsAttention: paneNeedsAttention(pane.title), LastSeen: time.Now().UTC(),
			}
			if item.Location != nil {
				location.WezTermPane = item.Location.WezTermPane
			}
			matches = append(matches, location)
		}
		if len(matches) == 1 {
			result[item.Key] = matches[0]
			claims[matches[0].TmuxPane] = append(claims[matches[0].TmuxPane], item)
		}
	}
	// A live process proves the pane is occupied, not which historical thread
	// owns it. Resolve competing leases only with a unique conversation title.
	for _, pane := range panes {
		owners := claims[pane.id]
		if len(owners) < 2 {
			continue
		}
		winner := ""
		matches := 0
		for _, owner := range owners {
			if samePath(owner.CWD, pane.cwd) && paneTitleMatches(conversationTitle(pane), owner.Title) {
				winner = owner.Key
				matches++
			}
		}
		for _, owner := range owners {
			if matches != 1 || owner.Key != winner {
				delete(result, owner.Key)
			}
		}
	}
	return result
}

// Codex appends the project name and optionally its path to the conversation.
// Strip that suffix before matching, so a cwd-derived title cannot claim it.
func conversationTitle(pane tmuxPane) string {
	parts := strings.Split(pane.title, "|")
	if len(parts) > 1 {
		last := strings.TrimSpace(parts[len(parts)-1])
		if strings.Contains(last, "/") {
			parts = parts[:len(parts)-1]
		}
	}
	if len(parts) > 1 && strings.TrimSpace(parts[len(parts)-1]) == filepath.Base(pane.cwd) {
		parts = parts[:len(parts)-1]
	}
	return strings.Join(parts, "|")
}

func paneNeedsAttention(title string) bool {
	return strings.Contains(strings.ToLower(title), "action required")
}

func parseTmuxPanes(data string) []tmuxPane {
	var panes []tmuxPane
	for _, line := range strings.Split(strings.TrimSpace(data), "\n") {
		parts := strings.SplitN(line, "\t", 4)
		if len(parts) != 4 || parts[0] == "" || process.NormalizeTTY(parts[1]) == "" {
			continue
		}
		panes = append(panes, tmuxPane{id: parts[0], tty: process.NormalizeTTY(parts[1]), cwd: parts[2], title: parts[3]})
	}
	return panes
}

func samePath(left, right string) bool {
	leftPath, leftErr := filepath.Abs(left)
	rightPath, rightErr := filepath.Abs(right)
	return leftErr == nil && rightErr == nil && filepath.Clean(leftPath) == filepath.Clean(rightPath)
}

func paneTitleMatches(paneTitle, sessionTitle string) bool {
	sessionTitle = strings.TrimSpace(sessionTitle)
	if sessionTitle == "" {
		return false
	}
	for _, part := range strings.Split(paneTitle, "|") {
		part = strings.TrimSpace(part)
		if part == sessionTitle || strings.HasSuffix(part, " "+sessionTitle) {
			return true
		}
	}
	return false
}
