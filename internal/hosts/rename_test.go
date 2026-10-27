package hosts

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/sky-bro/sesswitch/internal/process"
	"github.com/sky-bro/sesswitch/internal/session"
)

func TestRenamedThreadWithStaleTerminalTitle(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*session.Session)
		peer   *session.Session
		want   string
	}{
		{name: "verified unchanged lease", want: "current"},
		{name: "multiple renames", mutate: func(s *session.Session) { s.TitleAliases = []string{"Original", "Old"} }, want: "current"},
		{name: "no history", mutate: func(s *session.Session) { s.TitleAliases = nil }, want: "unknown"},
		{name: "unregistered", mutate: func(s *session.Session) { s.Location = nil }},
		{name: "no process identity", mutate: func(s *session.Session) { s.Location.AgentStart = "" }, want: "unknown"},
		{name: "changed process", mutate: func(s *session.Session) { s.Location.AgentPID = 99 }},
		{name: "reused PID", mutate: func(s *session.Session) { s.Location.AgentStart = "earlier" }},
		{name: "changed tty", mutate: func(s *session.Session) { s.Location.TTY = "/dev/ttys009" }, want: "unknown"},
		{name: "changed cwd", mutate: func(s *session.Session) { s.CWD = "/tmp/other" }, want: "unknown"},
		{name: "another thread displayed", mutate: func(s *session.Session) { s.TitleAliases = []string{"Different"} }, want: "unknown"},
		{name: "current title collision", peer: &session.Session{Key: "codex:peer", Provider: "codex", ID: "peer", Title: "Old", CWD: "/tmp/repo"}, want: "background"},
		{name: "historical title collision", peer: &session.Session{Key: "codex:peer", Provider: "codex", ID: "peer", Title: "Other", TitleAliases: []string{"Old"}, CWD: "/tmp/repo"}, want: "unknown"},
		{name: "stale peer still blocks collision", peer: &session.Session{Key: "codex:peer", Provider: "codex", ID: "peer", Title: "Old", CWD: "/tmp/repo", Location: &session.Location{TmuxPane: "%7", AgentPID: 1, AgentStart: "expired"}}, want: "unknown"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			item := session.Session{Key: "codex:a", Provider: "codex", ID: "a", Title: "New", TitleAliases: []string{"Old"}, CWD: "/tmp/repo",
				Location: &session.Location{TmuxPane: "%7", TTY: "/dev/ttys001", AgentPID: 42, AgentStart: "start"}}
			if tc.mutate != nil {
				tc.mutate(&item)
			}
			items := []session.Session{item}
			if tc.peer != nil {
				items = append(items, *tc.peer)
			}
			output := func(context.Context, string, ...string) ([]byte, error) {
				return []byte("%7\t/dev/ttys001\t/tmp/repo\t[ ! ] Action Required | Old | repo\n"), nil
			}
			agent := func(context.Context, string, string) (process.Info, bool, error) {
				return process.Info{PID: 42, Start: "start", TTY: "/dev/ttys001", Comm: "codex"}, true, nil
			}
			// A matching launch argv must not make history override a reused lease.
			thread := func(context.Context, process.Info, string, string) bool { return true }
			loc, ok := DiscoverTmuxLocations(context.Background(), items, output, agent, thread)[item.Key]
			if ok != (tc.want != "") || loc.DisplayState != tc.want {
				t.Fatalf("got %+v (found=%v), want %q", loc, ok, tc.want)
			}
			if tc.want == "current" {
				if loc.ThreadTitle != "Old" || !loc.NeedsAttention || items[0].Title != "New" {
					t.Fatalf("did not preserve visible and current names: %+v", loc)
				}
			} else if loc.NeedsAttention {
				t.Fatal("ambiguous/background session inherited pane attention")
			}
		})
	}
}

func TestFocusRenamedThreadRechecksActualTerminalTitle(t *testing.T) {
	ctx := context.Background()
	item := session.Session{Key: "codex:a", Provider: "codex", ID: "a", Title: "New", TitleAliases: []string{"Old"}, CWD: "/tmp/repo",
		Location: &session.Location{Provider: "codex", TmuxPane: "%7", TTY: "/dev/ttys001", AgentPID: 42, AgentStart: "start"}}
	title := "Old"
	output := func(_ context.Context, name string, args ...string) ([]byte, error) {
		switch name + " " + strings.Join(args, " ") {
		case "tmux list-panes -a -F #{pane_id}\t#{pane_tty}\t#{pane_current_path}\t#{pane_title}":
			return []byte("%7\t/dev/ttys001\t/tmp/repo\t" + title + " | repo\n"), nil
		case "tmux display-message -p -t %7 #{pane_current_path}\t#{pane_title}":
			return []byte("/tmp/repo\t" + title + " | repo\n"), nil
		case "tmux display-message -p -t %7 #{session_id}\t#{pane_tty}":
			return []byte("$1\t/dev/ttys001\n"), nil
		case "wezterm cli list --format json":
			return []byte(`[{"pane_id":3,"tty_name":"/dev/ttys002","is_active":true}]`), nil
		case "tmux list-clients -F #{client_tty}":
			return []byte("/dev/ttys002\n"), nil
		}
		return nil, errors.New("unexpected command")
	}
	agent := func(context.Context, string, string) (process.Info, bool, error) {
		return process.Info{PID: 42, Start: "start", TTY: "/dev/ttys001", Comm: "codex"}, true, nil
	}
	loc, ok := DiscoverTmuxLocations(ctx, []session.Session{item}, output, agent)[item.Key]
	if !ok || loc.DisplayState != "current" || loc.ThreadTitle != "Old" {
		t.Fatalf("renamed thread was not reconciled: %+v", loc)
	}
	var commands []string
	run := func(_ context.Context, name string, args ...string) error {
		commands = append(commands, name+" "+strings.Join(args, " "))
		return nil
	}
	verify := func(context.Context, session.Location, string) error { return nil }
	if err := Focus(ctx, loc, run, output, verify); err != nil {
		t.Fatal(err)
	}
	if len(commands) == 0 || commands[0] != "tmux switch-client -c /dev/ttys002 -t %7" {
		t.Fatalf("did not focus existing pane: %v", commands)
	}
	commands = nil
	title = "Different thread"
	if err := Focus(ctx, loc, run, output, verify); !errors.Is(err, ErrSessionNotDisplayed) || len(commands) != 0 {
		t.Fatalf("thread switch was ignored: err=%v commands=%v", err, commands)
	}
}
