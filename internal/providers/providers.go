package providers

import (
	"context"
	"fmt"
	"sort"

	"github.com/sky-bro/sesswitch/internal/session"
)

// Adapter discovers sessions for one agent implementation. Host coordinates
// are joined later so providers remain independent from terminals and editors.
type Adapter interface {
	Name() string
	List(context.Context, int) ([]session.Session, error)
}

type Result struct {
	Provider string
	Sessions []session.Session
	Err      error
}

// Collect queries providers independently and preserves adapter order in its
// results, so one unavailable provider cannot discard another one's catalog.
func Collect(ctx context.Context, adapters []Adapter, limit int) []Result {
	results := make([]Result, len(adapters))
	done := make(chan int, len(adapters))
	for i, adapter := range adapters {
		go func(index int, adapter Adapter) {
			items, err := adapter.List(ctx, limit)
			results[index] = Result{Provider: adapter.Name(), Sessions: items, Err: err}
			done <- index
		}(i, adapter)
	}
	for range adapters {
		<-done
	}
	return results
}

func List(ctx context.Context, adapters []Adapter, limit int) ([]session.Session, error) {
	if limit < 1 {
		return nil, fmt.Errorf("limit must be positive")
	}
	items := make([]session.Session, 0)
	seen := make(map[string]bool)
	for _, adapter := range adapters {
		providerItems, err := adapter.List(ctx, limit)
		if err != nil {
			return nil, fmt.Errorf("list %s sessions: %w", adapter.Name(), err)
		}
		for _, item := range providerItems {
			if item.Provider == "" {
				item.Provider = adapter.Name()
			}
			if item.Key == "" {
				item.Key = item.Provider + ":" + item.ID
			}
			if item.ID == "" || seen[item.Key] {
				continue
			}
			seen[item.Key] = true
			items = append(items, item)
		}
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].UpdatedAt.After(items[j].UpdatedAt) })
	if len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}
