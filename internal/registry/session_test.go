package registry

import (
	"testing"
	"time"

	"github.com/sky-bro/sesswitch/internal/session"
)

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
