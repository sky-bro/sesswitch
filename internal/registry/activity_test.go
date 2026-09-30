package registry

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sky-bro/sesswitch/internal/session"
)

func TestActivityRoundTripPrivate(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	store, err := New()
	if err != nil {
		t.Fatal(err)
	}
	if err := store.PutActivity("thread-1", session.Activity{Kind: "turn_ended", Source: "codex-hook:Stop"}); err != nil {
		t.Fatal(err)
	}
	items, err := store.Activities()
	if err != nil {
		t.Fatal(err)
	}
	if items["thread-1"].Kind != "turn_ended" || items["thread-1"].ObservedAt.IsZero() {
		t.Fatalf("unexpected activity: %+v", items["thread-1"])
	}
	info, err := os.Stat(filepath.Join(store.activityDir, "thread-1.json"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %v", info.Mode())
	}
	if err := store.PutActivity("../escape", session.Activity{Kind: "working"}); err == nil {
		t.Fatal("accepted unsafe session ID")
	}
}

func TestTaskMarkIndependentOfActivity(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	store, err := New()
	if err != nil {
		t.Fatal(err)
	}
	if err := store.PutTask("thread-1", session.Task{Kind: "done", Source: "user"}); err != nil {
		t.Fatal(err)
	}
	if err := store.PutActivity("thread-1", session.Activity{Kind: "closed", Source: "codex-hook:SessionEnd"}); err != nil {
		t.Fatal(err)
	}
	tasks, err := store.Tasks()
	if err != nil {
		t.Fatal(err)
	}
	if tasks["thread-1"].Kind != "done" {
		t.Fatalf("task lost after session end: %+v", tasks["thread-1"])
	}
	info, err := os.Stat(filepath.Join(store.taskDir, "thread-1.json"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %v", info.Mode())
	}
	if err := store.ClearTask("thread-1"); err != nil {
		t.Fatal(err)
	}
	tasks, err = store.Tasks()
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 0 {
		t.Fatalf("task not cleared: %+v", tasks)
	}
}

func TestNewMigratesLegacyStateDirectory(t *testing.T) {
	stateHome := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateHome)
	legacyLocations := filepath.Join(stateHome, "agent-locator", "locations")
	if err := os.MkdirAll(legacyLocations, 0o700); err != nil {
		t.Fatal(err)
	}
	legacyFile := filepath.Join(legacyLocations, "thread-1.json")
	if err := os.WriteFile(legacyFile, []byte(`{"kind":"tmux","tmux_pane":"%1"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := New()
	if err != nil {
		t.Fatal(err)
	}
	items, err := store.List()
	if err != nil {
		t.Fatal(err)
	}
	if items["thread-1"].TmuxPane != "%1" {
		t.Fatalf("legacy location was not migrated: %+v", items)
	}
	if _, err := os.Stat(filepath.Join(stateHome, "agent-locator")); !os.IsNotExist(err) {
		t.Fatalf("legacy directory remains: %v", err)
	}
}

func TestProviderKeySeparatesEqualSessionIDs(t *testing.T) {
	codex, err := Key("codex", "same-id")
	if err != nil {
		t.Fatal(err)
	}
	claude, err := Key("claude", "same-id")
	if err != nil {
		t.Fatal(err)
	}
	if codex == claude || codex != "codex--same-id" || claude != "claude--same-id" {
		t.Fatalf("unexpected keys: %q %q", codex, claude)
	}
}
