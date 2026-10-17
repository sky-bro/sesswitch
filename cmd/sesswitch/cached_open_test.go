package main

import (
	"context"
	"errors"
	"testing"

	"github.com/sky-bro/sesswitch/internal/hosts"
	"github.com/sky-bro/sesswitch/internal/session"
)

func TestCachedTmuxFocusReconcilesBeforeOpening(t *testing.T) {
	for _, tc := range []struct {
		name, source, resume, display string
		focusErr                      error
		wantFocus, wantErr            bool
	}{
		{name: "provider confirmed thread", source: "cli", display: "current", wantFocus: true},
		{name: "hook-only thread", source: "codex hook", display: "current"},
		{name: "non-resumable thread", source: "cli", resume: "unavailable", display: "current"},
		{name: "unknown provenance", display: "current"},
		{name: "another thread displayed", source: "cli", display: "background"},
		{name: "ambiguous ownership", source: "cli", display: "unknown"},
		{name: "exited after recovery", source: "cli", display: "current", focusErr: hosts.ErrStaleLocation},
		{name: "switched after recovery", source: "cli", display: "current", focusErr: hosts.ErrSessionNotDisplayed},
		{name: "activation failure does not resume", source: "cli", display: "current", focusErr: errors.New("activation denied"), wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			old := session.Location{Provider: "codex", TmuxPane: "%7", AgentPID: 1, AgentStart: "old", TTY: "/dev/ttys001", DisplayState: "current"}
			items := []session.Session{
				{Key: "codex:requested", Provider: "codex", ID: "requested", Source: tc.source, Resume: tc.resume, Location: &old},
				{Key: "codex:peer", Provider: "codex", ID: "peer"},
			}
			decorated, focusCalls := false, 0
			decorate := func(_ context.Context, got []session.Session) ([]session.Session, error) {
				decorated = true
				if len(got) != 2 || got[1].ID != "peer" {
					t.Fatal("competing peers omitted")
				}
				fresh := old
				fresh.AgentPID, fresh.AgentStart, fresh.DisplayState = 2, "new", tc.display
				got[0].Location = &fresh
				return got, nil
			}
			focus := func(_ context.Context, loc session.Location, _ hosts.Runner, _ hosts.OutputRunner, verify hosts.Verifier) error {
				focusCalls++
				if !decorated || loc.AgentPID != 2 || loc.AgentStart != "new" || verify == nil {
					t.Fatal("focused without fresh reconciliation and verification")
				}
				return tc.focusErr
			}
			focused, err := focusCachedTmuxLocationWith(context.Background(), "codex:requested", items, decorate, focus)
			if focused != tc.wantFocus || (err != nil) != tc.wantErr {
				t.Fatalf("focused=%v err=%v", focused, err)
			}
			eligible := tc.source != "" && tc.source != "codex hook" && tc.resume == ""
			if decorated != eligible {
				t.Fatalf("decorated=%v eligible=%v", decorated, eligible)
			}
			wantCalls := 0
			if eligible && tc.display == "current" {
				wantCalls = 1
			}
			if focusCalls != wantCalls {
				t.Fatalf("focus calls=%d want %d", focusCalls, wantCalls)
			}
		})
	}
}
