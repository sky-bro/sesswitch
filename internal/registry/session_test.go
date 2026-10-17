package registry

import (
	"testing"
	"time"

	"github.com/sky-bro/sesswitch/internal/session"
)

func TestResumeLimitationSurvivesHooksAndClearsOnProviderConfirmation(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	store, err := New()
	if err != nil {
		t.Fatal(err)
	}
	item := session.Session{Provider: "codex", ID: "side", Title: "Project", Source: "codex hook", UpdatedAt: time.Unix(123, 0)}
	if err := store.PutSession(item); err != nil {
		t.Fatal(err)
	}
	if err := store.SetResume("codex", "side", "unavailable"); err != nil {
		t.Fatal(err)
	}
	if err := store.PutSession(item); err != nil {
		t.Fatal(err)
	}
	items, err := store.Sessions("codex")
	if err != nil || len(items) != 1 || items[0].Resume != "unavailable" || !items[0].UpdatedAt.Equal(item.UpdatedAt) || items[0].Source != item.Source {
		t.Fatalf("lost limitation or changed activity: %+v %v", items, err)
	}
	item.Source = "cli"
	if err := store.PutSession(item); err != nil {
		t.Fatal(err)
	}
	items, err = store.Sessions("codex")
	if err != nil || items[0].Resume != "" {
		t.Fatalf("provider confirmation did not clear limitation: %+v %v", items, err)
	}
}

func TestSessionCatalogIsProviderScopedAndSorted(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	store, err := New()
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range []session.Session{
		{Provider: "claude", ID: "older", Title: "Older", UpdatedAt: time.Unix(1, 0)},
		{Provider: "codex", ID: "ignored", Title: "Codex", UpdatedAt: time.Unix(3, 0)},
		{Provider: "claude", ID: "newer", Title: "Newer", UpdatedAt: time.Unix(2, 0)},
	} {
		if err := store.PutSession(item); err != nil {
			t.Fatal(err)
		}
	}
	items, err := store.Sessions("claude")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || items[0].ID != "newer" || items[1].ID != "older" {
		t.Fatalf("unexpected catalog: %+v", items)
	}
}

func TestVerifiedHostSurvivesCloseAndCatalogRefresh(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	store, err := New()
	if err != nil {
		t.Fatal(err)
	}
	item := session.Session{Provider: "codex", ID: "thread", Title: "Example", Source: "vscode"}
	if err := store.PutSession(item); err != nil {
		t.Fatal(err)
	}
	if err := store.Put("codex--thread", session.Location{Provider: "codex", Kind: "tmux", TmuxPane: "%0", AgentPID: 42, AgentStart: "start", TTY: "/dev/ttys001"}); err != nil {
		t.Fatal(err)
	}
	if err := store.Delete("codex--thread"); err != nil {
		t.Fatal(err)
	}
	item.Status = "notLoaded"
	if err := store.PutSession(item); err != nil {
		t.Fatal(err)
	}
	items, err := store.Sessions("codex")
	if err != nil || len(items) != 1 || items[0].Host != "tmux" || items[0].Location != nil || items[0].Source != "vscode" {
		t.Fatalf("lost terminal provenance: %+v err=%v", items, err)
	}
}
