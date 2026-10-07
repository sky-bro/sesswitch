package state

import (
	"context"
	"slices"
	"strings"
	"time"

	"github.com/sky-bro/sesswitch/internal/process"
	"github.com/sky-bro/sesswitch/internal/session"
)

type Verifier func(context.Context, int, string, string) (process.Info, bool, error)

// Observed projects hook events without claiming a fresh process inspection.
// It is used by the event stream; live host checks happen on explicit snapshots.
func Observed(item session.Session) session.State {
	if item.Activity != nil {
		observed := item.Activity.ObservedAt
		return ApplyMarks(item, session.State{Kind: item.Activity.Kind, Source: item.Activity.Source, ObservedAt: &observed})
	}
	return ApplyMarks(item, item.State)
}

func ApplyMarks(item session.Session, resolved session.State) session.State {
	if item.Task != nil && item.Task.Kind == "read" && item.Task.ReadThrough != nil &&
		resolved.Kind == "turn_ended" && resolved.ObservedAt != nil &&
		!resolved.ObservedAt.After(*item.Task.ReadThrough) {
		resolved.Kind = "reviewed"
	}
	return resolved
}

func Resolve(ctx context.Context, item session.Session) session.State {
	return resolve(ctx, item, process.SameIdentityFor)
}

func ResolveWithVerifier(ctx context.Context, item session.Session, verify Verifier) session.State {
	return resolve(ctx, item, verify)
}

// ResolveDiscovered is only for a location verified in the current discovery
// pass. A persisted coordinate alone cannot establish that a session is open.
func ResolveDiscovered(ctx context.Context, item session.Session, location session.Location) session.State {
	resolved := Resolve(ctx, item)
	if resolved.Kind == "saved" {
		observed := location.LastSeen
		return session.State{Kind: "session_open", Source: "tmux-process", ObservedAt: &observed}
	}
	return resolved
}

func resolve(ctx context.Context, item session.Session, verify Verifier) session.State {
	if item.Status == "active" {
		if slices.Contains(item.ActiveFlags, "waitingOnApproval") {
			return session.State{Kind: "needs_approval", Source: "codex-app-server"}
		}
	}
	if item.Location != nil && item.Location.NeedsAttention {
		observed := item.Location.LastSeen
		return session.State{Kind: "needs_approval", Source: "tmux-title", ObservedAt: &observed}
	}
	if item.Status == "active" {
		return session.State{Kind: "working", Source: "codex-app-server"}
	}
	if activity := item.Activity; activity != nil {
		// A completed turn only needs review while its terminal agent is live.
		// GUI events can lack a CLI identity and retain their event semantics.
		if activity.Kind == "turn_ended" {
			pid, start, provider := activity.AgentPID, activity.AgentStart, activity.Provider
			if (pid <= 0 || start == "") && item.Location != nil {
				pid, start, provider = item.Location.AgentPID, item.Location.AgentStart, item.Location.Provider
			}
			if provider == "" {
				provider = item.Provider
			}
			if pid > 0 && start != "" {
				_, same, err := verify(ctx, pid, start, provider)
				if err != nil {
					return session.State{Kind: "saved", Source: "process-unverified"}
				}
				if !same {
					observed := time.Now().UTC()
					return session.State{Kind: "closed", Source: "process-exited", ObservedAt: &observed}
				}
			}
		}
		usable := true
		if activity.Kind == "needs_approval" || activity.Kind == "working" || activity.Kind == "session_open" {
			usable = activity.AgentPID > 0 && activity.AgentStart != ""
			if usable {
				_, same, err := verify(ctx, activity.AgentPID, activity.AgentStart, activity.Provider)
				usable = err == nil && same
			}
		}
		if usable {
			observed := activity.ObservedAt
			return session.State{Kind: activity.Kind, Source: activity.Source, ObservedAt: &observed}
		}
	}
	switch strings.ToLower(item.Status) {
	case "systemerror":
		return session.State{Kind: "error", Source: "codex-app-server"}
	case "idle":
		return session.State{Kind: "idle", Source: "codex-app-server"}
	default:
		return session.State{Kind: "saved", Source: "codex-app-server"}
	}
}

func Label(state session.State) string {
	switch state.Kind {
	case "needs_approval":
		return "needs approval"
	case "working":
		return "working"
	case "turn_ended":
		return "turn ended · review"
	case "reviewed":
		return "reviewed"
	case "session_open":
		return "session open"
	case "interrupted":
		return "interrupted"
	case "closed":
		return "session closed"
	case "error":
		return "error"
	case "idle":
		return "idle"
	default:
		return "saved"
	}
}

func Freshness(state session.State, now time.Time) string {
	if state.ObservedAt == nil || state.ObservedAt.IsZero() {
		return ""
	}
	if now.Sub(*state.ObservedAt) < time.Minute {
		return "just now"
	}
	return state.ObservedAt.Local().Format("Jan 2 15:04")
}
