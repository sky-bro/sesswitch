package hosts

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/sky-bro/sesswitch/internal/process"
	"github.com/sky-bro/sesswitch/internal/session"
)

func TestRecoverResumedTmuxThread(t *testing.T) {
	const id = "019a0000-1111-7222-8333-444444444444"
	base := session.Session{
		Key: "codex:" + id, ID: id, Provider: "codex", Title: "Same thread", CWD: "/tmp/repo",
		Location: &session.Location{Kind: "tmux", TmuxPane: "%366", TTY: "/dev/ttys001", AgentPID: 42, AgentStart: "old-start"},
		// Detached daemon hooks cannot replace the terminal location.
		Activity: &session.Activity{AgentPID: 88, AgentStart: "daemon", Kind: "working"},
	}
	live := process.Info{PID: 99, TTY: "/dev/ttys001", Start: "new-start", Comm: "codex"}
	for _, tc := range []struct {
		name   string
		mutate func(*session.Session)
		peer   *session.Session
		thread string
		want   bool
	}{
		{name: "same thread new CLI and daemon hook", thread: id, want: true},
		{name: "different thread reused pane", thread: "another-thread"},
		{name: "no thread identity evidence"},
		{name: "changed cwd", thread: id, mutate: func(s *session.Session) { s.CWD = "/tmp/other" }},
		{name: "changed title after interactive switch", thread: id, mutate: func(s *session.Session) { s.Title = "Other thread" }},
		{name: "same PID reused", thread: id, want: true, mutate: func(s *session.Session) { s.Location.AgentPID = 99 }},
		{name: "historical same title competitor", thread: id, peer: &session.Session{Key: "codex:other", Provider: "codex", Title: "Same thread", CWD: "/tmp/repo", Location: &session.Location{TmuxPane: "%366", AgentPID: 11, AgentStart: "older"}}},
		{name: "unregistered same title competitor", thread: id, peer: &session.Session{Key: "codex:other", Provider: "codex", Title: "Same thread", CWD: "/tmp/repo"}},
		{name: "new process already leased to another thread", thread: id, peer: &session.Session{Key: "codex:other", Provider: "codex", Title: "Other", CWD: "/tmp/repo", Location: &session.Location{TmuxPane: "%366", AgentPID: 99, AgentStart: "new-start"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			item := base
			loc := *base.Location
			item.Location = &loc
			if tc.mutate != nil {
				tc.mutate(&item)
			}
			items := []session.Session{item}
			if tc.peer != nil {
				items = append(items, *tc.peer)
			}
			output := func(context.Context, string, ...string) ([]byte, error) {
				return []byte("%366\t/dev/ttys001\t/tmp/repo\tSame thread | repo\n"), nil
			}
			agent := func(context.Context, string, string) (process.Info, bool, error) { return live, true, nil }
			thread := func(_ context.Context, got process.Info, provider, threadID string) bool {
				return got == live && provider == "codex" && threadID == tc.thread
			}
			got, ok := DiscoverTmuxLocations(context.Background(), items, output, agent, thread)[item.Key]
			if ok != tc.want {
				t.Fatalf("recovered=%v, want %v: %+v", ok, tc.want, got)
			}
			if ok && (got.AgentPID != live.PID || got.AgentStart != live.Start || got.DisplayState != "current") {
				t.Fatalf("identity was not replaced: %+v", got)
			}
			if item.Location.AgentStart != "old-start" {
				t.Fatal("discovery mutated original registry lease")
			}
		})
	}
}

func TestRecoveredLocationFocusesExistingPane(t *testing.T) {
	ctx := context.Background()
	item := session.Session{Key: "codex:id", ID: "id", Provider: "codex", Title: "Thread", CWD: "/tmp/repo",
		Location: &session.Location{Kind: "tmux", TmuxPane: "%7", AgentPID: 1, AgentStart: "old"}}
	live := process.Info{PID: 2, TTY: "/dev/ttys001", Start: "new", Comm: "codex"}
	output := func(_ context.Context, name string, args ...string) ([]byte, error) {
		switch name + " " + strings.Join(args, " ") {
		case "tmux list-panes -a -F #{pane_id}\t#{pane_tty}\t#{pane_current_path}\t#{pane_title}":
			return []byte("%7\t/dev/ttys001\t/tmp/repo\tThread | repo\n"), nil
		case "tmux display-message -p -t %7 #{pane_current_path}\t#{pane_title}":
			return []byte("/tmp/repo\tThread | repo\n"), nil
		case "tmux display-message -p -t %7 #{session_id}\t#{pane_tty}":
			return []byte("$1\t/dev/ttys001\n"), nil
		case "wezterm cli list --format json":
			return []byte(`[{"pane_id":3,"tty_name":"/dev/ttys002","is_active":true}]`), nil
		case "tmux list-clients -F #{client_tty}":
			return []byte("/dev/ttys002\n"), nil
		}
		return nil, errors.New("unexpected output request")
	}
	agent := func(context.Context, string, string) (process.Info, bool, error) { return live, true, nil }
	thread := func(context.Context, process.Info, string, string) bool { return true }
	loc, ok := DiscoverTmuxLocations(ctx, []session.Session{item}, output, agent, thread)[item.Key]
	if !ok {
		t.Fatal("recovery failed")
	}
	var commands []string
	run := func(_ context.Context, name string, args ...string) error {
		commands = append(commands, name+" "+strings.Join(args, " "))
		return nil
	}
	verify := func(_ context.Context, loc session.Location, tty string) error {
		if loc.AgentPID != live.PID || loc.AgentStart != live.Start || tty != live.TTY {
			return ErrStaleLocation
		}
		return nil
	}
	if err := Focus(ctx, loc, run, output, verify); err != nil {
		t.Fatal(err)
	}
	if len(commands) == 0 || commands[0] != "tmux switch-client -c /dev/ttys002 -t %7" {
		t.Fatalf("wrong focus: %v", commands)
	}
	for _, cmd := range commands {
		if strings.Contains(cmd, "resume") || strings.Contains(cmd, "spawn") || strings.HasPrefix(cmd, "wezterm start") {
			t.Fatalf("unexpected new CLI: %s", cmd)
		}
	}
	// The recovered lease must still fail if the new process exits before focus.
	commands = nil
	if err := Focus(ctx, loc, run, output, func(context.Context, session.Location, string) error { return ErrStaleLocation }); !errors.Is(err, ErrStaleLocation) || len(commands) != 0 {
		t.Fatalf("stale recovered process was focused: err=%v, commands=%v", err, commands)
	}
}
