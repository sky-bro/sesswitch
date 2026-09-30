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
