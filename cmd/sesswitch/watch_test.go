package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/sky-bro/sesswitch/internal/providers"
	"github.com/sky-bro/sesswitch/internal/registry"
	"github.com/sky-bro/sesswitch/internal/session"
)

func TestWatchStreamsHookChangesWithoutRefreshingProvider(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	store, err := registry.New()
	if err != nil {
		t.Fatal(err)
	}
	item := session.Session{Provider: "codex", ID: "one", Title: "Example", Status: "notLoaded"}
	if err := store.PutSession(item); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	reader, writer := io.Pipe()
	defer reader.Close()
	done := make(chan error, 1)
	calls := 0
	go func() {
		err := streamSnapshots(ctx, 1, writer, func(context.Context, int) (snapshot, error) {
			calls++
			items, err := localCatalog(store, 1)
			return snapshot{Sessions: items}, err
		})
		writer.Close()
		done <- err
	}()
	updates := make(chan snapshot)
	go func() {
		decoder := json.NewDecoder(reader)
		for {
			var value snapshot
			if decoder.Decode(&value) != nil {
				return
			}
			select {
			case updates <- value:
			case <-ctx.Done():
				return
			}
		}
	}()
	next := func() snapshot {
		t.Helper()
		select {
		case value := <-updates:
			return value
		case <-time.After(3 * time.Second):
			t.Fatal("no filesystem update received")
			return snapshot{}
		}
	}
	if len(next().Sessions) != 1 {
		t.Fatal("missing initial snapshot")
	}
	if err := store.PutActivity("codex--one", session.Activity{Provider: "codex", Kind: "needs_approval", Source: "codex-hook:PermissionRequest"}); err != nil {
		t.Fatal(err)
	}
	if got := next().Sessions[0].State.Kind; got != "needs_approval" {
		t.Fatalf("got %s", got)
	}
	if err := store.PutActivity("codex--one", session.Activity{Provider: "codex", Kind: "turn_ended", Source: "codex-hook:Stop"}); err != nil {
		t.Fatal(err)
	}
	turn := next().Sessions[0].Activity.ObservedAt
	if err := store.PutTask("codex--one", session.Task{Kind: "read", ReadThrough: &turn}); err != nil {
		t.Fatal(err)
	}
	if got := next().Sessions[0].State.Kind; got != "reviewed" {
		t.Fatalf("got %s", got)
	}
	if err := store.PutActivity("codex--one", session.Activity{Provider: "codex", Kind: "turn_ended", Source: "codex-hook:Stop"}); err != nil {
		t.Fatal(err)
	}
	if got := next().Sessions[0].State.Kind; got != "turn_ended" {
		t.Fatalf("new turn was suppressed: %s", got)
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("provider refreshed %d times", calls)
	}
}

type snapshotAdapter struct {
	name  string
	items []session.Session
	err   error
}

func (a snapshotAdapter) Name() string                                         { return a.name }
func (a snapshotAdapter) List(context.Context, int) ([]session.Session, error) { return a.items, a.err }

func TestProviderFailureRetainsCacheAndOtherProvider(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	store, _ := registry.New()
	if err := store.PutSession(session.Session{Provider: "codex", ID: "cached", Title: "Cached Codex"}); err != nil {
		t.Fatal(err)
	}
	got, err := loadWithAdapters(context.Background(), 10, []providers.Adapter{
		snapshotAdapter{name: "codex", err: errors.New("offline")},
		snapshotAdapter{name: "claude", items: []session.Session{{Provider: "claude", ID: "fresh", Title: "Fresh Claude"}}},
	})
	if err != nil || len(got.Sessions) != 2 || len(got.Warnings) != 1 {
		t.Fatalf("snapshot=%+v err=%v", got, err)
	}
}

func TestHistoryLimitDoesNotHideHookObservedSession(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	store, _ := registry.New()
	for _, item := range []session.Session{
		{Provider: "codex", ID: "old", UpdatedAt: time.Unix(1, 0)},
		{Provider: "codex", ID: "new", UpdatedAt: time.Unix(2, 0)},
		{Provider: "claude", ID: "latest", UpdatedAt: time.Unix(3, 0)},
	} {
		if err := store.PutSession(item); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.PutActivity("codex--old", session.Activity{Kind: "needs_approval"}); err != nil {
		t.Fatal(err)
	}
	items, err := localCatalog(store, 1)
	if err != nil || len(items) != 2 || items[0].Key != "claude:latest" || items[1].Key != "codex:old" {
		t.Fatalf("items=%+v err=%v", items, err)
	}
}
