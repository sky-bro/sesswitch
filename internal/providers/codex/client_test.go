package codex

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConvert(t *testing.T) {
	name := "Named thread"
	originator := "codex-tui"
	items := convert([]thread{
		{ID: "a", SessionID: "tree", Name: &name, CWD: "/tmp/repo", UpdatedAt: 10, Status: json.RawMessage(`{"type":"active","activeFlags":["waitingOnApproval"]}`), Source: json.RawMessage(`"cli"`)},
		{ID: "b", Preview: "  First   prompt\nsecond line", CWD: "/tmp/other", UpdatedAt: 20, Status: json.RawMessage(`{"type":"notLoaded"}`), Source: json.RawMessage(`{"vscode":{}}`), Originator: &originator},
	})
	if items[0].Title != "Named thread" || items[0].Status != "active" || len(items[0].ActiveFlags) != 1 || items[0].ActiveFlags[0] != "waitingOnApproval" || items[0].Source != "cli" {
		t.Fatalf("unexpected first item: %#v", items[0])
	}
	if items[1].Title != "First prompt" || items[1].Source != "vscode" {
		t.Fatalf("unexpected second item: %#v", items[1])
	}
}

func TestSourceNamePrefersSpecificHostOriginator(t *testing.T) {
	tests := []struct {
		name       string
		source     json.RawMessage
		originator string
		want       string
	}{
		{
			name:       "chrome extension overrides generic vscode source",
			source:     json.RawMessage(`"vscode"`),
			originator: "codex-chrome-extension-sidepanel",
			want:       "codex-chrome-extension-sidepanel",
		},
		{
			name:       "desktop overrides generic vscode source",
			source:     json.RawMessage(`"vscode"`),
			originator: "Codex Desktop",
			want:       "Codex Desktop",
		},
		{
			name:       "tui does not override vscode source",
			source:     json.RawMessage(`"vscode"`),
			originator: "codex-tui",
			want:       "vscode",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := sourceName(test.source, &test.originator); got != test.want {
				t.Fatalf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestBrowserTabFromRolloutUsesLatestSelectedTab(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rollout.jsonl")
	contents := `{"type":"session_meta","payload":{"id":"abc"}}
{"type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"# Chrome tabs:\n- [selected] Tab ID 10: https://example.com/old"}]}}
{"type":"response_item","payload":{"type":"message","role":"assistant","content":[{"type":"output_text","text":"ignore"}]}}
{"type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"# Chrome tabs:\n  - [selected] Tab ID 20: https://example.com/new\n\nrequest"}]}}
`
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	id, url, err := browserTabFromRollout(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if id != "20" || url != "https://example.com/new" {
		t.Fatalf("got tab %q at %q", id, url)
	}
}

func TestRenameValidation(t *testing.T) {
	if err := Rename(context.Background(), "", "New name"); err == nil {
		t.Fatal("accepted an empty thread ID")
	}
	if err := Rename(context.Background(), "thread-1", "   "); err == nil {
		t.Fatal("accepted an empty thread name")
	}
}

func TestReadValidation(t *testing.T) {
	if _, err := Read(context.Background(), "   "); err == nil {
		t.Fatal("accepted an empty thread ID")
	}
}

// The helper process rejects catalog scans and transcript requests. It exercises
// the actual stdio handshake used by Read, including app-server errors.
func TestReadAppServerHelper(t *testing.T) {
	if os.Getenv("SESSWITCH_READ_HELPER") != "1" {
		return
	}
	scanner := bufio.NewScanner(os.Stdin)
	encoder := json.NewEncoder(os.Stdout)
	for scanner.Scan() {
		var req struct {
			Method string `json:"method"`
			ID     int    `json:"id"`
			Params struct {
				ThreadID     string `json:"threadId"`
				IncludeTurns bool   `json:"includeTurns"`
			} `json:"params"`
		}
		if json.Unmarshal(scanner.Bytes(), &req) != nil {
			os.Exit(2)
		}
		switch req.Method {
		case "initialize":
			_ = encoder.Encode(map[string]any{"id": req.ID, "result": map[string]any{}})
		case "initialized":
		case "thread/read":
			if req.Params.IncludeTurns || req.Params.ThreadID != "thread-1" {
				os.Exit(3)
			}
			if os.Getenv("SESSWITCH_READ_FAIL") == "1" {
				_ = encoder.Encode(map[string]any{"id": req.ID, "error": map[string]any{"code": -1, "message": "thread missing"}})
			} else {
				_ = encoder.Encode(map[string]any{"id": req.ID, "result": map[string]any{"thread": map[string]any{"id": "thread-1", "name": "A thread", "source": "vscode", "originator": "codex-chrome-extension-sidepanel", "path": os.Getenv("SESSWITCH_READ_ROLLOUT")}}})
			}
			os.Exit(0)
		default:
			os.Exit(4)
		}
	}
	os.Exit(5)
}

func TestReadUsesExactThreadAndBrowserContext(t *testing.T) {
	directory := t.TempDir()
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	launcher := filepath.Join(directory, "codex")
	quoted := "'" + strings.ReplaceAll(binary, "'", "'\"'\"'") + "'"
	if err := os.WriteFile(launcher, []byte("#!/bin/sh\nexec "+quoted+" -test.run=^TestReadAppServerHelper$\n"), 0700); err != nil {
		t.Fatal(err)
	}
	rollout := filepath.Join(directory, "rollout.jsonl")
	contents := `{"type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"# Chrome tabs:\n- [selected] Tab ID 12: https://example.com/"}]}}` + "\n"
	if err := os.WriteFile(rollout, []byte(contents), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SESSWITCH_CODEX", launcher)
	t.Setenv("SESSWITCH_READ_HELPER", "1")
	t.Setenv("SESSWITCH_READ_ROLLOUT", rollout)
	item, err := Read(context.Background(), "thread-1")
	if err != nil {
		t.Fatal(err)
	}
	if item.ID != "thread-1" || item.Title != "A thread" || item.BrowserTabID != "12" || item.BrowserURL != "https://example.com/" {
		t.Fatalf("unexpected thread: %+v", item)
	}
	t.Setenv("SESSWITCH_READ_FAIL", "1")
	if _, err := Read(context.Background(), "thread-1"); err == nil || !strings.Contains(err.Error(), "thread missing") {
		t.Fatalf("lost RPC error: %v", err)
	}
}

func TestBrowserContextFallbacks(t *testing.T) {
	for _, test := range []struct {
		name, text, role string
		want             string
	}{
		{"unrelated text", "- [selected] Tab ID 1: https://example.com/", "user", ""},
		{"assistant context", "# Chrome tabs:\n- [selected] Tab ID 1: https://example.com/", "assistant", ""},
		{"ambiguous", "# Chrome tabs:\n- [selected] Tab ID 1: https://example.com/\n- [selected] Tab ID 2: https://example.com/", "user", ""},
		{"valid", "# Chrome tabs:\n- [selected] Tab ID 3: https://example.com/", "user", "3"},
	} {
		t.Run(test.name, func(t *testing.T) {
			data, _ := json.Marshal(map[string]any{"type": "response_item", "payload": map[string]any{"type": "message", "role": test.role, "content": []map[string]string{{"type": "input_text", "text": test.text}}}})
			path := filepath.Join(t.TempDir(), "rollout.jsonl")
			if err := os.WriteFile(path, data, 0600); err != nil {
				t.Fatal(err)
			}
			id, _, err := browserTabFromRollout(context.Background(), path)
			if err != nil || id != test.want {
				t.Fatal(fmt.Sprintf("id=%q err=%v", id, err))
			}
		})
	}
	if _, _, err := browserTabFromRollout(context.Background(), ""); err == nil {
		t.Fatal("accepted missing rollout")
	}
}
