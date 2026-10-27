package codex

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/sky-bro/sesswitch/internal/session"
)

func TestTitleHistoryUsesSameThreadAndLatestName(t *testing.T) {
	home := t.TempDir()
	t.Setenv("CODEX_HOME", home)
	data := `{"id":"a","thread_name":"Original"}
{"id":"other","thread_name":"Unrelated"}
{"id":"a","thread_name":"Intermediate"}
invalid record
{"id":"a","thread_name":"Original"}
{"id":"a","thread_name":"Current"}
{"id":"stale","thread_name":"Old"}
{"id":"stale","thread_name":"Newer than catalog"}
{"id":"cleared","thread_name":"Old"}
{"id":"cleared","thread_name":""}
`
	if err := os.WriteFile(filepath.Join(home, "session_index.jsonl"), []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	items := []session.Session{
		{ID: "a", Provider: "codex", Title: "Current"},
		{ID: "stale", Provider: "codex", Title: "Old"},
		{ID: "missing", Provider: "codex", Title: "Current"},
		{ID: "a", Provider: "claude", Title: "Current"},
		{ID: "cleared", Provider: "codex", Title: "Old"},
	}
	AddTitleHistory(context.Background(), items)
	if !reflect.DeepEqual(items[0].TitleAliases, []string{"Original", "Intermediate"}) {
		t.Fatalf("unexpected aliases: %v", items[0].TitleAliases)
	}
	for _, item := range items[1:] {
		if len(item.TitleAliases) != 0 {
			t.Fatalf("unverified history attached: %+v", item)
		}
	}
	encoded, err := json.Marshal(items[0])
	if err != nil || strings.Contains(string(encoded), "Original") {
		t.Fatalf("historical titles serialized: %s, %v", encoded, err)
	}
}

func TestTitleHistoryOptionalMetadata(t *testing.T) {
	for _, mode := range []string{"missing", "truncated", "canceled", "default home"} {
		t.Run(mode, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("CODEX_HOME", home)
			if mode == "default home" {
				t.Setenv("HOME", home)
				t.Setenv("CODEX_HOME", "")
				home = filepath.Join(home, ".codex")
				if err := os.Mkdir(home, 0o700); err != nil {
					t.Fatal(err)
				}
			}
			data := "{\"id\":\"a\",\"thread_name\":\"Old\"}\n{\"id\":\"a\",\"thread_name\":\"Current\"}\n"
			if mode == "truncated" {
				data += strings.Repeat("x", 1024*1024+1)
			}
			if mode != "missing" {
				if err := os.WriteFile(filepath.Join(home, "session_index.jsonl"), []byte(data), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if mode == "canceled" {
				cancel()
			}
			items := []session.Session{{Provider: "codex", ID: "a", Title: "Current", TitleAliases: []string{"Stale cached alias"}}}
			AddTitleHistory(ctx, items)
			want := 0
			if mode == "default home" {
				want = 1
			}
			if len(items[0].TitleAliases) != want {
				t.Fatalf("unexpected history: %v", items[0].TitleAliases)
			}
		})
	}
}
