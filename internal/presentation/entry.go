package presentation

import (
	"path/filepath"
	"strings"

	"github.com/sky-bro/sesswitch/internal/session"
)

// Entry is launcher-neutral display data. Text-only launchers can use Line;
// native launchers can map the individual fields to title, subtitle, icon,
// and accessories without parsing a rendered string.
type Entry struct {
	StatusIcon   string
	ProviderIcon string
	Title        string
	Project      string
	Provider     string
	Destination  string
	Status       string
}

func New(item session.Session) Entry {
	icon, status := statePresentation(item)
	return Entry{
		StatusIcon: icon, ProviderIcon: providerIcon(item.Provider),
		Title: oneLine(item.Title), Project: projectName(item.CWD),
		Provider: providerName(item.Provider), Destination: destination(item), Status: status,
	}
}

func (entry Entry) Line() string {
	main := strings.TrimSpace(strings.Join(nonEmpty(entry.StatusIcon, entry.ProviderIcon, entry.Title), "  "))
	var metadata []string
	if entry.Project != "" {
		metadata = append(metadata, entry.Project)
	}
	route := strings.TrimSpace(entry.Provider)
	if entry.Destination != "" {
		if route != "" {
			route += " › "
		}
		route += entry.Destination
	}
	if route != "" {
		metadata = append(metadata, route)
	}
	if entry.Status != "" {
		metadata = append(metadata, entry.Status)
	}
	if len(metadata) == 0 {
		return main
	}
	return main + "  —  " + strings.Join(metadata, "  ·  ")
}

func nonEmpty(values ...string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			result = append(result, value)
		}
	}
	return result
}

func statePresentation(item session.Session) (string, string) {
	if item.Task != nil && item.Task.Kind == "done" {
		return "✅", "Done"
	}
	switch item.State.Kind {
	case "needs_approval":
		return "🟠", "Action needed"
	case "working":
		return "🔵", "Working"
	case "turn_ended":
		return "🟢", "Ready to review"
	case "reviewed":
		return "⚪", "Reviewed"
	case "session_open":
		return "🟣", "Open"
	case "interrupted":
		return "🔴", "Interrupted"
	case "closed":
		return "⚫", "Closed"
	case "error":
		return "🔴", "Error"
	case "idle":
		return "🟡", "Idle"
	default:
		return "⚪", "Saved"
	}
}

func providerName(provider string) string {
	switch strings.ToLower(provider) {
	case "codex":
		return "Codex"
	case "claude":
		return "Claude"
	default:
		return provider
	}
}

func providerIcon(provider string) string {
	switch strings.ToLower(provider) {
	case "codex":
		return "⬡"
	case "claude":
		return "✦"
	default:
		return "•"
	}
}

func destination(item session.Session) string {
	if item.Location != nil {
		switch strings.ToLower(item.Location.Kind) {
		case "tmux":
			return "tmux"
		case "wezterm":
			return "WezTerm"
		default:
			return item.Location.Kind
		}
	}
	source := strings.ToLower(item.Source)
	switch {
	case strings.Contains(source, "desktop"):
		return "Desktop"
	case strings.Contains(source, "chrome"):
		return "Chrome"
	case strings.Contains(source, "vscode"), strings.Contains(source, "vs code"):
		return "VS Code"
	default:
		return "Terminal"
	}
}

func projectName(cwd string) string {
	if cwd == "" {
		return ""
	}
	name := filepath.Base(filepath.Clean(cwd))
	if name == "." || name == string(filepath.Separator) {
		return ""
	}
	return name
}

func oneLine(value string) string {
	return strings.Join(strings.Fields(value), " ")
}
