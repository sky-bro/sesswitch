package state

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sky-bro/sesswitch/internal/process"
	"github.com/sky-bro/sesswitch/internal/session"
)

func TestAppServerApprovalWins(t *testing.T) {
	item := session.Session{Status: "active", ActiveFlags: []string{"waitingOnApproval"}, Activity: &session.Activity{Kind: "turn_ended"}}
	got := resolve(context.Background(), item, nil)
	if got.Kind != "needs_approval" || got.Source != "codex-app-server" {
		t.Fatalf("unexpected state: %+v", got)
	}
}

func TestHookStatusSurvivesNotLoaded(t *testing.T) {
	now := time.Now().UTC()
	item := session.Session{Status: "notLoaded", Activity: &session.Activity{Kind: "needs_approval", Source: "codex-hook:PermissionRequest", ObservedAt: now, AgentPID: 123, AgentStart: "start"}}
	verify := func(context.Context, int, string, string) (process.Info, bool, error) {
		return process.Info{}, true, nil
	}
	got := resolve(context.Background(), item, verify)
	if got.Kind != "needs_approval" || got.ObservedAt == nil || *got.ObservedAt != now {
		t.Fatalf("unexpected state: %+v", got)
	}
}

func TestDeadOrUninspectableProcessCannotAppearActive(t *testing.T) {
	item := session.Session{Status: "notLoaded", Activity: &session.Activity{Kind: "needs_approval", AgentPID: 123, AgentStart: "start"}}
	for _, verify := range []Verifier{
		func(context.Context, int, string, string) (process.Info, bool, error) {
			return process.Info{}, false, nil
		},
		func(context.Context, int, string, string) (process.Info, bool, error) {
			return process.Info{}, false, errors.New("ps failed")
		},
	} {
		if got := resolve(context.Background(), item, verify); got.Kind != "saved" {
			t.Fatalf("unexpected state: %+v", got)
		}
	}
}

func TestOldSessionOpenCannotAppearLive(t *testing.T) {
	item := session.Session{Status: "notLoaded", Activity: &session.Activity{Kind: "session_open", AgentPID: 123, AgentStart: "start"}}
	verify := func(context.Context, int, string, string) (process.Info, bool, error) {
		return process.Info{}, false, nil
	}
	if got := resolve(context.Background(), item, verify); got.Kind != "saved" {
		t.Fatalf("unexpected state: %+v", got)
	}
}

func TestTurnEndedIsNotTaskCompleted(t *testing.T) {
	item := session.Session{Status: "notLoaded", Activity: &session.Activity{Kind: "turn_ended", Source: "codex-hook:Stop"}}
	got := resolve(context.Background(), item, nil)
	if got.Kind != "turn_ended" || Label(got) != "turn ended · review" {
		t.Fatalf("unexpected state: %+v", got)
	}
}

func TestLiveTmuxActionRequiredOverridesTurnEndedHook(t *testing.T) {
	now := time.Now().UTC()
	item := session.Session{
		Status:   "notLoaded",
		Activity: &session.Activity{Kind: "turn_ended", Source: "codex-hook:Stop"},
		Location: &session.Location{Kind: "tmux", NeedsAttention: true, LastSeen: now},
	}
	got := resolve(context.Background(), item, nil)
	if got.Kind != "needs_approval" || got.Source != "tmux-title" || got.ObservedAt == nil || !got.ObservedAt.Equal(now) {
		t.Fatalf("unexpected state: %+v", got)
	}
}
