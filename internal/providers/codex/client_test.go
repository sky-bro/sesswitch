package codex

import (
	"context"
	"encoding/json"
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

func TestRenameValidation(t *testing.T) {
	if err := Rename(context.Background(), "", "New name"); err == nil {
		t.Fatal("accepted an empty thread ID")
	}
	if err := Rename(context.Background(), "thread-1", "   "); err == nil {
		t.Fatal("accepted an empty thread name")
	}
}
