package open

import (
	"context"
	"errors"
	"reflect"
	"runtime"
	"testing"
	"time"

	"github.com/sky-bro/sesswitch/internal/session"
)

func acceptLocation(context.Context, session.Location, string) error { return nil }

func TestCommandEnvironmentDropsStaleWezTermSocket(t *testing.T) {
	environment := []string{"PATH=/bin", "WEZTERM_UNIX_SOCKET=/tmp/stale", "OTHER=value"}
	got := commandEnvironment("wezterm", environment, func(string) bool { return false })
	want := []string{"PATH=/bin", "OTHER=value"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestCommandEnvironmentKeepsLiveWezTermSocket(t *testing.T) {
	environment := []string{"PATH=/bin", "WEZTERM_UNIX_SOCKET=/tmp/live"}
	got := commandEnvironment("wezterm", environment, func(path string) bool { return path == "/tmp/live" })
	if !reflect.DeepEqual(got, environment) {
		t.Fatalf("got %q, want original environment", got)
	}
}

func TestCommandEnvironmentDoesNotTouchOtherCommands(t *testing.T) {
	environment := []string{"WEZTERM_UNIX_SOCKET=/tmp/stale"}
	got := commandEnvironment("tmux", environment, func(string) bool { return false })
	if !reflect.DeepEqual(got, environment) {
		t.Fatalf("got %q, want original environment", got)
	}
}

func TestResumeInWezTerm(t *testing.T) {
	var got []string
	run := func(_ context.Context, name string, args ...string) error {
		got = append([]string{name}, args...)
		return nil
	}
	item := session.Session{Provider: "codex", ID: "abc", CWD: "/tmp/repo"}
	if err := Session(context.Background(), item, "terminal", run); err != nil {
		t.Fatal(err)
	}
	want := []string{"wezterm", "start", "--cwd", "/tmp/repo", "--", "codex", "resume", "abc"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestResumeClaudeInWezTerm(t *testing.T) {
	var got []string
	run := func(_ context.Context, name string, args ...string) error {
		got = append([]string{name}, args...)
		return nil
	}
	item := session.Session{Provider: "claude", ID: "abc", CWD: "/tmp/repo"}
	if err := Session(context.Background(), item, "terminal", run); err != nil {
		t.Fatal(err)
	}
	want := []string{"wezterm", "start", "--cwd", "/tmp/repo", "--", "claude", "--resume", "abc"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestFocusTmuxPaneThroughItsWezTermClient(t *testing.T) {
	var calls [][]string
	run := func(_ context.Context, name string, args ...string) error {
		calls = append(calls, append([]string{name}, args...))
		return nil
	}
	output := func(_ context.Context, name string, args ...string) ([]byte, error) {
		calls = append(calls, append([]string{name}, args...))
		switch name {
		case "tmux":
			if args[0] == "display-message" {
				return []byte("work\t/dev/ttys015\n"), nil
			}
			return []byte("/dev/ttys000\n"), nil
		default:
			return []byte(`[{"window_id":2,"pane_id":7,"tty_name":"/dev/ttys000","is_active":true}]`), nil
		}
	}
	item := session.Session{
		Provider: "codex",
		ID:       "abc",
		Location: &session.Location{Kind: "tmux", TmuxPane: "%17", WezTermPane: "7", LastSeen: time.Now()},
	}
	if err := sessionWithRunners(context.Background(), item, "auto", run, output, acceptLocation); err != nil {
		t.Fatal(err)
	}
	want := [][]string{
		{"tmux", "display-message", "-p", "-t", "%17", "#{session_name}\t#{pane_tty}"},
		{"wezterm", "cli", "list", "--format", "json"},
		{"tmux", "list-clients", "-F", "#{client_tty}"},
		{"tmux", "switch-client", "-c", "/dev/ttys000", "-t", "%17"},
		{"wezterm", "cli", "activate-pane", "--pane-id", "7"},
	}
	if runtime.GOOS == "darwin" {
		want = append(want, []string{"open", "-a", "WezTerm"})
	}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("got %q, want %q", calls, want)
	}
}

func TestDetachedTmuxSessionAttachesInExistingWezTermWindow(t *testing.T) {
	var calls [][]string
	run := func(_ context.Context, name string, args ...string) error {
		calls = append(calls, append([]string{name}, args...))
		return nil
	}
	output := func(_ context.Context, name string, args ...string) ([]byte, error) {
		calls = append(calls, append([]string{name}, args...))
		if name == "tmux" {
			if args[0] == "display-message" {
				return []byte("detached\t/dev/ttys015\n"), nil
			}
			return nil, context.Canceled
		}
		if args[1] == "list" {
			return []byte(`[{"window_id":2,"pane_id":9,"tty_name":"/dev/ttys001","is_active":true}]`), nil
		}
		return []byte("12\n"), nil
	}
	item := session.Session{
		Provider: "codex",
		ID:       "abc",
		Location: &session.Location{Kind: "tmux", TmuxPane: "%17", WezTermPane: "7", LastSeen: time.Now()},
	}
	if err := sessionWithRunners(context.Background(), item, "auto", run, output, acceptLocation); err != nil {
		t.Fatal(err)
	}
	want := [][]string{
		{"tmux", "display-message", "-p", "-t", "%17", "#{session_name}\t#{pane_tty}"},
		{"wezterm", "cli", "list", "--format", "json"},
		{"tmux", "list-clients", "-F", "#{client_tty}"},
		{"tmux", "select-window", "-t", "%17"},
		{"tmux", "select-pane", "-t", "%17"},
		{"wezterm", "cli", "spawn", "--window-id", "2", "--", "tmux", "attach-session", "-t", "detached"},
		{"wezterm", "cli", "activate-pane", "--pane-id", "12"},
	}
	if runtime.GOOS == "darwin" {
		want = append(want, []string{"open", "-a", "WezTerm"})
	}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("got %q, want %q", calls, want)
	}
}

func TestActivateWezTermPaneRaisesAppOnMacOS(t *testing.T) {
	var calls [][]string
	run := func(_ context.Context, name string, args ...string) error {
		calls = append(calls, append([]string{name}, args...))
		return nil
	}
	if err := activateWezTermPane(context.Background(), "7", run); err != nil {
		t.Fatal(err)
	}
	want := [][]string{{"wezterm", "cli", "activate-pane", "--pane-id", "7"}}
	if runtime.GOOS == "darwin" {
		want = append(want, []string{"open", "-a", "WezTerm"})
	}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("got %q, want %q", calls, want)
	}
}

func TestDetachedTmuxSessionStartsWezTermOnlyWhenNoneExists(t *testing.T) {
	var calls [][]string
	run := func(_ context.Context, name string, args ...string) error {
		calls = append(calls, append([]string{name}, args...))
		return nil
	}
	output := func(_ context.Context, name string, args ...string) ([]byte, error) {
		calls = append(calls, append([]string{name}, args...))
		if name == "tmux" && args[0] == "display-message" {
			return []byte("detached\t/dev/ttys015\n"), nil
		}
		return nil, errors.New("no GUI/client")
	}
	item := session.Session{
		Provider: "codex",
		ID:       "abc",
		Location: &session.Location{Kind: "tmux", TmuxPane: "%17", LastSeen: time.Now()},
	}
	if err := sessionWithRunners(context.Background(), item, "auto", run, output, acceptLocation); err != nil {
		t.Fatal(err)
	}
	wantLast := []string{"wezterm", "start", "--", "tmux", "attach-session", "-t", "detached"}
	if got := calls[len(calls)-1]; !reflect.DeepEqual(got, wantLast) {
		t.Fatalf("last call got %q, want %q", got, wantLast)
	}
}

func TestFocusFailureDoesNotResumeDuplicate(t *testing.T) {
	var calls [][]string
	run := func(_ context.Context, name string, args ...string) error {
		calls = append(calls, append([]string{name}, args...))
		if name == "tmux" {
			return errors.New("switch failed")
		}
		return nil
	}
	output := func(_ context.Context, name string, args ...string) ([]byte, error) {
		if name == "tmux" && args[0] == "display-message" {
			return []byte("work\t/dev/ttys015\n"), nil
		}
		if name == "tmux" {
			return []byte("/dev/ttys000\n"), nil
		}
		return []byte(`[{"window_id":2,"pane_id":7,"tty_name":"/dev/ttys000","is_active":true}]`), nil
	}
	item := session.Session{Provider: "codex", ID: "abc", Location: &session.Location{Kind: "tmux", TmuxPane: "%17"}}
	err := sessionWithRunners(context.Background(), item, "auto", run, output, acceptLocation)
	if err == nil || len(calls) != 1 || calls[0][0] != "tmux" {
		t.Fatalf("unexpected fallback: calls=%v err=%v", calls, err)
	}
}

func TestStaleLocationResumes(t *testing.T) {
	var calls [][]string
	run := func(_ context.Context, name string, args ...string) error {
		calls = append(calls, append([]string{name}, args...))
		return nil
	}
	output := func(_ context.Context, _ string, _ ...string) ([]byte, error) {
		return []byte("work\t/dev/ttys015\n"), nil
	}
	item := session.Session{Provider: "codex", ID: "abc", Location: &session.Location{Kind: "tmux", TmuxPane: "%17"}}
	verify := func(context.Context, session.Location, string) error { return ErrStaleLocation }
	if err := sessionWithRunners(context.Background(), item, "auto", run, output, verify); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 1 || calls[0][0] != "wezterm" {
		t.Fatalf("unexpected calls: %v", calls)
	}
}

func TestDeadRegisteredProcessResumesEvenAfterPaneRemoved(t *testing.T) {
	var got []string
	run := func(_ context.Context, name string, args ...string) error {
		got = append([]string{name}, args...)
		return nil
	}
	output := func(context.Context, string, ...string) ([]byte, error) {
		t.Fatal("pane lookup should not run for a dead registered process")
		return nil, nil
	}
	item := session.Session{Provider: "codex", ID: "abc", Location: &session.Location{Kind: "tmux", TmuxPane: "%17", TTY: "/dev/ttys015", AgentPID: 123, AgentStart: "old"}}
	verify := func(context.Context, session.Location, string) error { return ErrStaleLocation }
	if err := sessionWithRunners(context.Background(), item, "auto", run, output, verify); err != nil {
		t.Fatal(err)
	}
	if len(got) < 2 || got[0] != "wezterm" {
		t.Fatalf("unexpected resume command: %v", got)
	}
}

func TestAutoRoutesDesktopThread(t *testing.T) {
	var got []string
	run := func(_ context.Context, name string, args ...string) error {
		got = append([]string{name}, args...)
		return nil
	}
	item := session.Session{Provider: "codex", ID: "abc", Source: "Codex Desktop"}
	if err := sessionWithRunners(context.Background(), item, "auto", run, nil, nil); err != nil {
		t.Fatal(err)
	}
	want := []string{"open", "codex://threads/abc"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestAutoRoutesChromeAndVSCodeToTheirApps(t *testing.T) {
	tests := []struct {
		source string
		bundle string
	}{
		{"codex-chrome-extension-sidepanel", "com.google.Chrome"},
		{"codex_vscode", "com.microsoft.VSCode"},
	}
	for _, test := range tests {
		t.Run(test.source, func(t *testing.T) {
			var got []string
			run := func(_ context.Context, name string, args ...string) error {
				got = append([]string{name}, args...)
				return nil
			}
			item := session.Session{Provider: "codex", ID: "abc", Source: test.source}
			if err := sessionWithRunners(context.Background(), item, "auto", run, nil, nil); err != nil {
				t.Fatal(err)
			}
			want := []string{"open", "-b", test.bundle}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("got %q, want %q", got, want)
			}
		})
	}
}

func TestAutoFocusesRecordedChromeTab(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Chrome tab focus uses macOS AppleScript")
	}
	var got []string
	run := func(_ context.Context, name string, args ...string) error {
		got = append([]string{name}, args...)
		return nil
	}
	item := session.Session{
		Provider:     "codex",
		ID:           "abc",
		Source:       "codex-chrome-extension-sidepanel",
		BrowserTabID: "1882448824",
		BrowserURL:   "https://example.com/project",
	}
	if err := sessionWithRunners(context.Background(), item, "auto", run, nil, nil); err != nil {
		t.Fatal(err)
	}
	if len(got) < 6 || got[0] != "osascript" || got[len(got)-2] != item.BrowserTabID || got[len(got)-1] != item.BrowserURL {
		t.Fatalf("unexpected focus command: %q", got)
	}
}

func TestAutoFallsBackToChromeWhenRecordedTabIsUnavailable(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Chrome tab focus uses macOS AppleScript")
	}
	var calls [][]string
	run := func(_ context.Context, name string, args ...string) error {
		calls = append(calls, append([]string{name}, args...))
		if name == "osascript" {
			return errors.New("tab not found")
		}
		return nil
	}
	item := session.Session{
		Provider:     "codex",
		ID:           "abc",
		Source:       "codex-chrome-extension-sidepanel",
		BrowserTabID: "1882448824",
	}
	if err := sessionWithRunners(context.Background(), item, "auto", run, nil, nil); err != nil {
		t.Fatal(err)
	}
	wantLast := []string{"open", "-b", "com.google.Chrome"}
	if len(calls) != 2 || !reflect.DeepEqual(calls[1], wantLast) {
		t.Fatalf("unexpected fallback calls: %q", calls)
	}
}
