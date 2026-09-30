package providers

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sky-bro/sesswitch/internal/session"
)

type fakeAdapter struct {
	name  string
	items []session.Session
	err   error
}

func (f fakeAdapter) Name() string                                         { return f.name }
func (f fakeAdapter) List(context.Context, int) ([]session.Session, error) { return f.items, f.err }

func TestListNormalizesMergesAndLimits(t *testing.T) {
	items, err := List(context.Background(), []Adapter{
		fakeAdapter{name: "codex", items: []session.Session{{ID: "a", UpdatedAt: time.Unix(1, 0)}}},
		fakeAdapter{name: "claude", items: []session.Session{{ID: "b", UpdatedAt: time.Unix(3, 0)}, {ID: "c", UpdatedAt: time.Unix(2, 0)}}},
	}, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || items[0].Key != "claude:b" || items[1].Key != "claude:c" {
		t.Fatalf("unexpected items: %+v", items)
	}
}

func TestListAttributesProviderError(t *testing.T) {
	_, err := List(context.Background(), []Adapter{fakeAdapter{name: "claude", err: errors.New("bad index")}}, 10)
	if err == nil || err.Error() != "list claude sessions: bad index" {
		t.Fatalf("unexpected error: %v", err)
	}
}
