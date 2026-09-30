package main

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/sky-bro/sesswitch/internal/hosts"
	"github.com/sky-bro/sesswitch/internal/registry"
	"github.com/sky-bro/sesswitch/internal/session"
)

func TestTmuxPaneAtTTY(t *testing.T) {
	output := func(_ context.Context, name string, args ...string) ([]byte, error) {
		if name != "tmux" || args[0] != "list-panes" {
			t.Fatalf("unexpected call: %s %v", name, args)
		}
		return []byte("/dev/ttys014\t%16\n/dev/ttys015\t%17\n"), nil
	}
	pane, err := tmuxPaneAtTTY(context.Background(), "/dev/ttys015", output)
	if err != nil || pane != "%17" {
		t.Fatalf("pane=%q err=%v", pane, err)
	}
	_, err = tmuxPaneAtTTY(context.Background(), "/dev/ttys016", output)
	if err == nil {
		t.Fatal("expected missing pane error")
	}
	_, err = tmuxPaneAtTTY(context.Background(), "/dev/ttys015", func(context.Context, string, ...string) ([]byte, error) { return nil, errors.New("tmux unavailable") })
	if err == nil {
		t.Fatal("expected tmux error")
	}
}

func TestHookActivityKinds(t *testing.T) {
	tests := []struct {
		event      string
		source     string
		stopActive bool
		want       string
	}{
		{"SessionStart", "startup", false, "session_open"},
		{"SessionStart", "compact", false, ""},
		{"UserPromptSubmit", "", false, "working"},
		{"PermissionRequest", "", false, "needs_approval"},
		{"PostToolUse", "", false, "working"},
		{"Stop", "", false, "turn_ended"},
		{"Stop", "", true, "working"},
		{"Interrupt", "", false, "interrupted"},
		{"StopFailure", "", false, "interrupted"},
		{"SessionEnd", "", false, "closed"},
	}
	for _, test := range tests {
		input := hookInput{HookEventName: test.event, Source: test.source, StopHookActive: test.stopActive}
		if got := hookActivityKind(input); got != test.want {
			t.Errorf("%s: got %q, want %q", test.event, got, test.want)
		}
	}
}

func TestMarkCommand(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	if err := markCommand([]string{"done", "codex:thread-1"}); err != nil {
		t.Fatal(err)
	}
	if err := markCommand([]string{"clear", "codex:thread-1"}); err != nil {
		t.Fatal(err)
	}
	if err := markCommand([]string{"done", "claude:thread-1"}); err != nil {
		t.Fatal(err)
	}
	if err := markCommand([]string{"done", "codex:../escape"}); err == nil {
		t.Fatal("accepted unsafe ID")
	}
}

func TestRenameCommandValidation(t *testing.T) {
	if err := renameCommand([]string{"claude:thread-1", "New name"}); err == nil {
		t.Fatal("accepted rename for an unsupported provider")
	}
	if err := renameCommand([]string{"codex:thread-1", "  "}); err == nil {
		t.Fatal("accepted an empty name")
	}
	if err := renameCommand([]string{"missing-separator", "New name"}); err == nil {
		t.Fatal("accepted an invalid session key")
	}
}

func TestRememberLocation(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	item := session.Session{
		Provider: "codex",
		ID:       "thread-1",
		Location: &session.Location{
			Provider: "codex", Kind: "tmux", TmuxPane: "%7", TTY: "/dev/ttys007",
			AgentPID: 42, AgentStart: "start",
		},
	}
	if err := rememberLocation(item); err != nil {
		t.Fatal(err)
	}
	store, err := registry.New()
	if err != nil {
		t.Fatal(err)
	}
	locations, err := store.List()
	if err != nil {
		t.Fatal(err)
	}
	if got := locations["codex--thread-1"]; got.TmuxPane != "%7" || got.AgentPID != 42 {
		t.Fatalf("unexpected remembered location: %+v", got)
	}
}

func TestHookWritesTurnObservationAndSessionClosure(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	if err := hookCommand([]string{"codex"}, strings.NewReader(`{"session_id":"thread-1","hook_event_name":"Stop","turn_id":"turn-2"}`)); err != nil {
		t.Fatal(err)
	}
	store, err := registry.New()
	if err != nil {
		t.Fatal(err)
	}
	activities, err := store.Activities()
	if err != nil {
		t.Fatal(err)
	}
	if got := activities["codex--thread-1"]; got.Kind != "turn_ended" || got.TurnID != "turn-2" {
		t.Fatalf("unexpected stop activity: %+v", got)
	}
	if err := hookCommand([]string{"codex"}, strings.NewReader(`{"session_id":"thread-1","hook_event_name":"SessionEnd"}`)); err != nil {
		t.Fatal(err)
	}
	activities, err = store.Activities()
	if err != nil {
		t.Fatal(err)
	}
	if got := activities["codex--thread-1"]; got.Kind != "closed" {
		t.Fatalf("unexpected closure: %+v", got)
	}
}

func TestClaudeHookIndexesSessionWithoutTranscript(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	start := `{"session_id":"claude-1","hook_event_name":"SessionStart","source":"startup","cwd":"/tmp/example","session_title":"Fix tests","transcript_path":"/private/sensitive.jsonl"}`
	if err := hookCommand([]string{"claude"}, strings.NewReader(start)); err != nil {
		t.Fatal(err)
	}
	if err := hookCommand([]string{"claude"}, strings.NewReader(`{"session_id":"claude-1","hook_event_name":"PermissionRequest","cwd":"/tmp/example"}`)); err != nil {
		t.Fatal(err)
	}
	store, err := registry.New()
	if err != nil {
		t.Fatal(err)
	}
	items, err := store.Sessions("claude")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Title != "Fix tests" || items[0].CWD != "/tmp/example" {
		t.Fatalf("unexpected Claude catalog: %+v", items)
	}
	activities, err := store.Activities()
	if err != nil {
		t.Fatal(err)
	}
	if got := activities["claude--claude-1"]; got.Kind != "needs_approval" {
		t.Fatalf("unexpected Claude activity: %+v", got)
	}
}

func TestRegisteredLocationFastPath(t *testing.T) {
	for _, test := range []struct {
		name     string
		focusErr error
		want     bool
		wantErr  bool
	}{
		{"focused", nil, true, false},
		{"stale falls back", hosts.ErrStaleLocation, false, false},
		{"focus failure stops", errors.New("activation denied"), false, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("XDG_STATE_HOME", t.TempDir())
			store, err := registry.New()
			if err != nil {
				t.Fatal(err)
			}
			loc := session.Location{Provider: "codex", TmuxPane: "%7", TTY: "/dev/ttys007", AgentPID: 42, AgentStart: "start"}
			if err := store.Put("codex--thread-1", loc); err != nil {
				t.Fatal(err)
			}
			calls := 0
			focus := func(ctx context.Context, got session.Location, run hosts.Runner, output hosts.OutputRunner, verify hosts.Verifier) error {
				calls++
				if got.AgentPID != 42 || verify == nil {
					t.Fatal("missing process verification")
				}
				return test.focusErr
			}
			got, err := focusRegisteredLocationWith(context.Background(), "codex", "thread-1", focus)
			if got != test.want || (err != nil) != test.wantErr || calls != 1 {
				t.Fatalf("focused=%v err=%v calls=%d", got, err, calls)
			}
		})
	}
}

func TestRegisteredLocationRejectsUnverifiedOrWrongProvider(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	store, err := registry.New()
	if err != nil {
		t.Fatal(err)
	}
	focus := func(context.Context, session.Location, hosts.Runner, hosts.OutputRunner, hosts.Verifier) error {
		t.Fatal("unverified location focused")
		return nil
	}
	for _, loc := range []session.Location{
		{Provider: "codex", TmuxPane: "%7"},
		{Provider: "claude", TmuxPane: "%7", TTY: "/dev/ttys007", AgentPID: 42, AgentStart: "start"},
	} {
		if err := store.Put("codex--thread-1", loc); err != nil {
			t.Fatal(err)
		}
		got, err := focusRegisteredLocationWith(context.Background(), "codex", "thread-1", focus)
		if got || err != nil {
			t.Fatalf("focused=%v err=%v", got, err)
		}
	}
}

func TestOpenKeyValidationBeforeLookup(t *testing.T) {
	for _, key := range []string{"bad", "unknown:id", "codex:../escape", "codex:id:extra"} {
		if err := openKey(key, "auto"); err == nil {
			t.Fatalf("accepted %q", key)
		}
	}
	if err := openKey("codex:id", "invalid"); err == nil {
		t.Fatal("accepted invalid target")
	}
}
