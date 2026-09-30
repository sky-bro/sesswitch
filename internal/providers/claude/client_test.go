package claude

import (
	"context"
	"testing"

	"github.com/sky-bro/sesswitch/internal/registry"
	"github.com/sky-bro/sesswitch/internal/session"
)

func TestListHookObservedSessions(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	store, err := registry.New()
	if err != nil {
		t.Fatal(err)
	}
	if err := store.PutSession(session.Session{Provider: "claude", ID: "abc", Title: "Claude session"}); err != nil {
		t.Fatal(err)
	}
	items, err := New().List(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Key != "claude:abc" {
		t.Fatalf("unexpected items: %+v", items)
	}
}
