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

func TestTerminalReviewRequiresLiveProcess(t *testing.T) {
	for _, fromLocation := range []bool{false, true} {
		for _, test := range []struct {
			name string
			same bool
			err  error
			want string
		}{
			{"live or detached", true, nil, "turn_ended"},
			{"exited or PID reused", false, nil, "closed"},
			{"inspection unavailable", false, errors.New("ps failed"), "saved"},
		} {
			t.Run(test.name, func(t *testing.T) {
				item := session.Session{Provider: "codex", Status: "notLoaded",
					Activity: &session.Activity{Kind: "turn_ended", Source: "codex-hook:Stop"}}
				if fromLocation {
					item.Location = &session.Location{Kind: "tmux", AgentPID: 123, AgentStart: "start"}
				} else {
					item.Activity.AgentPID, item.Activity.AgentStart = 123, "start"
				}
				verify := func(_ context.Context, pid int, start, provider string) (process.Info, bool, error) {
					if pid != 123 || start != "start" || provider != "codex" {
						t.Fatalf("wrong identity: %d %s %s", pid, start, provider)
					}
					return process.Info{}, test.same, test.err
				}
				if got := resolve(context.Background(), item, verify); got.Kind != test.want {
					t.Fatalf("got %+v, want %s", got, test.want)
				}
			})
		}
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

func TestFreshDiscoveryOnlyPromotesSaved(t *testing.T) {
	now := time.Now().UTC()
	location := session.Location{LastSeen: now}
	for _, test := range []struct {
		name, status, activity, want string
		attention                    bool
	}{
		{"no hooks", "notLoaded", "", "session_open", false},
		{"working", "active", "", "working", false},
		{"approval", "notLoaded", "", "needs_approval", true},
		{"review", "notLoaded", "turn_ended", "turn_ended", false},
		{"closed", "notLoaded", "closed", "closed", false},
		{"error", "systemError", "", "error", false},
		{"idle", "idle", "", "idle", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			loc := location
			loc.NeedsAttention = test.attention
			item := session.Session{Status: test.status, Location: &loc}
			if test.activity != "" {
				item.Activity = &session.Activity{Kind: test.activity}
			}
			got := ResolveDiscovered(context.Background(), item, loc)
			if got.Kind != test.want {
				t.Fatalf("got %+v, want %s", got, test.want)
			}
			if test.want == "session_open" && (got.Source != "tmux-process" || got.ObservedAt == nil || !got.ObservedAt.Equal(now)) {
				t.Fatalf("bad provenance: %+v", got)
			}
		})
	}
	item := session.Session{Status: "notLoaded", Location: &location}
	if got := Resolve(context.Background(), item); got.Kind != "saved" {
		t.Fatalf("persisted location promoted: %+v", got)
	}
}
