package main

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/sky-bro/sesswitch/internal/providers"
	"github.com/sky-bro/sesswitch/internal/providers/claude"
	"github.com/sky-bro/sesswitch/internal/providers/codex"
	"github.com/sky-bro/sesswitch/internal/registry"
	"github.com/sky-bro/sesswitch/internal/session"
	"github.com/sky-bro/sesswitch/internal/state"
)

type snapshot struct {
	Sessions  []session.Session `json:"sessions"`
	Warnings  []string          `json:"warnings,omitempty"`
	CatalogAt time.Time         `json:"catalog_at"`
}

// Refresh catalogs only at open/manual refresh. Hooks update the local catalog
// and observations independently; a failed provider retains its cached metadata.
func loadSnapshot(ctx context.Context, limit int) (snapshot, error) {
	return loadWithAdapters(ctx, limit, []providers.Adapter{codex.New(), claude.New()})
}

func loadWithAdapters(ctx context.Context, limit int, adapters []providers.Adapter) (snapshot, error) {
	store, err := registry.New()
	if err != nil {
		return snapshot{}, err
	}
	result := snapshot{CatalogAt: time.Now().UTC()}
	for _, provider := range providers.Collect(ctx, adapters, limit) {
		if provider.Err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("%s: %v (using cached sessions)", provider.Provider, provider.Err))
			continue
		}
		for _, item := range provider.Sessions {
			if item.Provider == "" {
				item.Provider = provider.Provider
			}
			if err := store.PutSession(item); err != nil {
				return result, err
			}
		}
	}
	items, err := localCatalog(store, limit)
	if err != nil {
		return result, err
	}
	result.Sessions, err = decorateSessions(ctx, items)
	return result, err
}

// limit applies to history, never to hook-observed sessions still needing
// attention or running. Include both providers before applying the history cap.
func localCatalog(store *registry.Store, limit int) ([]session.Session, error) {
	var items []session.Session
	for _, provider := range []string{"codex", "claude"} {
		part, err := store.Sessions(provider)
		if err != nil {
			return nil, err
		}
		items = append(items, part...)
	}
	activities, err := store.Activities()
	if err != nil {
		return nil, err
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].UpdatedAt.After(items[j].UpdatedAt) })
	result := make([]session.Session, 0, len(items))
	history := 0
	for _, item := range items {
		activity, _ := lookupRecord(activities, item)
		active := activity.Kind != "" && activity.Kind != "closed"
		if active || history < limit {
			result = append(result, item)
			if !active {
				history++
			}
		}
	}
	return result, nil
}

// projectLocal performs no provider queries, process probes, or registry writes.
func projectLocal(store *registry.Store, limit int, previous snapshot) (snapshot, error) {
	items, err := localCatalog(store, limit)
	if err != nil {
		return previous, err
	}
	activities, err := store.Activities()
	if err != nil {
		return previous, err
	}
	locations, err := store.List()
	if err != nil {
		return previous, err
	}
	tasks, err := store.Tasks()
	if err != nil {
		return previous, err
	}
	prior := make(map[string]session.Session, len(previous.Sessions))
	for _, item := range previous.Sessions {
		prior[item.Key] = item
	}
	for i := range items {
		item := &items[i]
		if activity, found := lookupRecord(activities, *item); found {
			item.Activity = &activity
		}
		if location, found := lookupRecord(locations, *item); found {
			item.Location = &location
		}
		if task, found := lookupRecord(tasks, *item); found {
			item.Task = &task
		}
		old, known := prior[item.Key]
		if known && sameActivity(old.Activity, item.Activity) {
			// Preserve the last explicit host observation, including a title-based
			// approval, until this session receives a new hook or is refreshed.
			item.State = old.State
			if item.State.Kind == "reviewed" && item.Activity != nil {
				item.State.Kind = item.Activity.Kind
			}
			item.State = state.ApplyMarks(*item, item.State)
		} else {
			item.State = state.Observed(*item)
		}
		if item.State.Kind == "" {
			item.State = session.State{Kind: "saved", Source: "catalog"}
		}
		// New work reopens a completed task. SessionEnd alone does not.
		if item.Task != nil && item.Task.Kind == "done" && item.Activity != nil &&
			item.Activity.Kind != "closed" && item.Activity.ObservedAt.After(item.Task.UpdatedAt) {
			item.Task = nil
		}
	}
	previous.Sessions = items
	return previous, nil
}

func sameActivity(a, b *session.Activity) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func recordKey(path string) bool { return strings.HasSuffix(path, ".json") }

// Provider-qualified records take precedence over the pre-namespace registry.
func lookupRecord[T any](records map[string]T, item session.Session) (T, bool) {
	for _, id := range []string{item.ID, item.SessionID} {
		if id == "" {
			continue
		}
		key, err := registry.Key(item.Provider, id)
		if err == nil {
			if value, found := records[key]; found {
				return value, true
			}
		}
		if value, found := records[id]; found {
			return value, true
		}
	}
	var zero T
	return zero, false
}
