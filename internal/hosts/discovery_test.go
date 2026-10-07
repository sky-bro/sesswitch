package hosts

import (
	"context"
	"errors"
	"testing"

	"github.com/sky-bro/sesswitch/internal/process"
	"github.com/sky-bro/sesswitch/internal/session"
)

func TestDiscoverTmuxLocationFromTitleCWDAndProcess(t *testing.T) {
	items := []session.Session{{Key: "codex:abc", Provider: "codex", Title: "调研微信通知与控制方案", CWD: "/tmp/home-lab"}}
	output := func(context.Context, string, ...string) ([]byte, error) {
		return []byte("%0\t/dev/ttys001\t/tmp/home-lab\t⠦ 调研微信通知与控制方案 | home-lab\n"), nil
	}
	agent := func(_ context.Context, tty, provider string) (process.Info, bool, error) {
		if tty != "/dev/ttys001" || provider != "codex" {
			t.Fatalf("unexpected lookup %q %q", tty, provider)
		}
		return process.Info{PID: 42, TTY: tty, Start: "start", Comm: "codex"}, true, nil
	}
	got := DiscoverTmuxLocations(context.Background(), items, output, agent)
	location, ok := got["codex:abc"]
	if !ok || location.TmuxPane != "%0" || location.AgentPID != 42 || location.AgentStart != "start" {
		t.Fatalf("unexpected location: %#v", got)
	}
}

func TestDiscoverTmuxLocationRejectsAmbiguousMatches(t *testing.T) {
	items := []session.Session{{Key: "codex:abc", Provider: "codex", Title: "Same title", CWD: "/tmp/repo"}}
	output := func(context.Context, string, ...string) ([]byte, error) {
		return []byte("%1\t/dev/ttys001\t/tmp/repo\tSame title | repo\n%2\t/dev/ttys002\t/tmp/repo\tSame title | repo\n"), nil
	}
	agent := func(_ context.Context, tty, _ string) (process.Info, bool, error) {
		return process.Info{PID: 42, TTY: tty, Start: "start", Comm: "codex"}, true, nil
	}
	if got := DiscoverTmuxLocations(context.Background(), items, output, agent); len(got) != 0 {
		t.Fatalf("ambiguous pane should not be linked: %#v", got)
	}
}

func TestDiscoverTmuxLocationRequiresLiveAgent(t *testing.T) {
	items := []session.Session{{Key: "codex:abc", Provider: "codex", Title: "A title", CWD: "/tmp/repo"}}
	output := func(context.Context, string, ...string) ([]byte, error) {
		return []byte("%1\t/dev/ttys001\t/tmp/repo\tA title | repo\n"), nil
	}
	agent := func(context.Context, string, string) (process.Info, bool, error) {
		return process.Info{}, false, errors.New("no agent")
	}
	if got := DiscoverTmuxLocations(context.Background(), items, output, agent); len(got) != 0 {
		t.Fatalf("pane without agent should not be linked: %#v", got)
	}
}

func TestDiscoverTmuxRefreshesActionRequiredOnRegisteredLocation(t *testing.T) {
	items := []session.Session{{
		Key: "codex:abc", Provider: "codex", Title: "Approve command", CWD: "/tmp/repo",
		Location: &session.Location{Kind: "tmux", TmuxPane: "%7", WezTermPane: "3"},
	}}
	output := func(context.Context, string, ...string) ([]byte, error) {
		return []byte("%7\t/dev/ttys001\t/tmp/repo\t[ . ] Action Required | Approve command | repo\n"), nil
	}
	agent := func(_ context.Context, tty, _ string) (process.Info, bool, error) {
		return process.Info{PID: 42, TTY: tty, Start: "start", Comm: "codex"}, true, nil
	}
	location := DiscoverTmuxLocations(context.Background(), items, output, agent)["codex:abc"]
	if !location.NeedsAttention || location.TmuxPane != "%7" || location.WezTermPane != "3" {
		t.Fatalf("unexpected refreshed location: %+v", location)
	}
}

func TestDiscoverTmuxRefreshesRenamedRegisteredLocation(t *testing.T) {
	items := []session.Session{{
		Key: "codex:abc", Provider: "codex", Title: "微信+HA", CWD: "/tmp/repo",
		Location: &session.Location{
			Kind: "tmux", TmuxPane: "%7", TTY: "/dev/ttys001",
			AgentPID: 42, AgentStart: "start", NeedsAttention: true,
		},
	}}
	output := func(context.Context, string, ...string) ([]byte, error) {
		return []byte("%7\t/dev/ttys001\t/tmp/repo\t⠹ Old title | repo\n"), nil
	}
	agent := func(_ context.Context, tty, _ string) (process.Info, bool, error) {
		return process.Info{PID: 42, TTY: tty, Start: "start", Comm: "codex"}, true, nil
	}
	location, ok := DiscoverTmuxLocations(context.Background(), items, output, agent)["codex:abc"]
	if !ok || location.NeedsAttention || location.TmuxPane != "%7" {
		t.Fatalf("renamed registered pane was not refreshed: %+v", location)
	}
}

func TestDiscoverTmuxRejectsReusedRegisteredPane(t *testing.T) {
	items := []session.Session{{
		Key: "codex:abc", Provider: "codex", Title: "Renamed", CWD: "/tmp/repo",
		Location: &session.Location{
			Kind: "tmux", TmuxPane: "%7", TTY: "/dev/ttys001",
			AgentPID: 42, AgentStart: "old-start",
		},
	}}
	output := func(context.Context, string, ...string) ([]byte, error) {
		return []byte("%7\t/dev/ttys001\t/tmp/repo\tUnrelated title\n"), nil
	}
	agent := func(_ context.Context, tty, _ string) (process.Info, bool, error) {
		return process.Info{PID: 99, TTY: tty, Start: "new-start", Comm: "codex"}, true, nil
	}
	if got := DiscoverTmuxLocations(context.Background(), items, output, agent); len(got) != 0 {
		t.Fatalf("reused pane should not refresh a stale lease: %#v", got)
	}
}

func TestDiscoverTmuxProjectSuffixDoesNotClaimPane(t *testing.T) {
	for _, title := range []string{
		"⠸ 比较米家与HA自动化依赖 | home-lab",
		"[ ! ] Action Required | 比较米家与HA自动化依赖 | home-lab | ~/repos/home-lab",
	} {
		t.Run(title, func(t *testing.T) {
			items := []session.Session{
				{Key: "codex:old", Provider: "codex", Title: "home-lab", CWD: "/tmp/home-lab"},
				{Key: "codex:current", Provider: "codex", Title: "比较米家与HA自动化依赖", CWD: "/tmp/home-lab"},
			}
			output := func(context.Context, string, ...string) ([]byte, error) {
				return []byte("%0\t/dev/ttys001\t/tmp/home-lab\t" + title + "\n"), nil
			}
			agent := func(_ context.Context, tty, _ string) (process.Info, bool, error) {
				return process.Info{PID: 42, TTY: tty, Start: "start"}, true, nil
			}
			for _, registered := range []bool{false, true} {
				if registered {
					for i := range items {
						items[i].Location = &session.Location{Kind: "tmux", TmuxPane: "%0", AgentPID: 42, AgentStart: "start"}
					}
				}
				got := DiscoverTmuxLocations(context.Background(), items, output, agent)
				if len(got) != 1 || got["codex:current"].TmuxPane != "%0" {
					t.Fatalf("registered=%v: unexpected claims: %#v", registered, got)
				}
			}
		})
	}
}

func TestDiscoverTmuxRejectsCompetingThreadsWithSameTitle(t *testing.T) {
	items := []session.Session{
		{Key: "codex:a", Provider: "codex", Title: "Same", CWD: "/tmp/repo"},
		{Key: "codex:b", Provider: "codex", Title: "Same", CWD: "/tmp/repo"},
	}
	output := func(context.Context, string, ...string) ([]byte, error) {
		return []byte("%0\t/dev/ttys001\t/tmp/repo\tSame | repo\n"), nil
	}
	agent := func(_ context.Context, tty, _ string) (process.Info, bool, error) {
		return process.Info{PID: 42, TTY: tty, Start: "start"}, true, nil
	}
	if got := DiscoverTmuxLocations(context.Background(), items, output, agent); len(got) != 0 {
		t.Fatalf("competing threads must not share a pane: %#v", got)
	}
}
