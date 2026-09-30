package claude

import (
	"context"

	"github.com/sky-bro/sesswitch/internal/registry"
	"github.com/sky-bro/sesswitch/internal/session"
)

// Adapter lists sessions observed through Claude Code's documented hook API.
// It intentionally does not parse Claude's private transcript format.
type Adapter struct{}

func New() Adapter { return Adapter{} }

func (Adapter) Name() string { return "claude" }

func (Adapter) List(_ context.Context, limit int) ([]session.Session, error) {
	store, err := registry.New()
	if err != nil {
		return nil, err
	}
	items, err := store.Sessions("claude")
	if err != nil {
		return nil, err
	}
	if len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}
