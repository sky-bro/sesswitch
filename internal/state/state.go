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

func Resolve(ctx context.Context, item session.Session) session.State {
	return resolve(ctx, item, process.SameIdentityFor)
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
