package presentation

import (
	"testing"

	"github.com/sky-bro/sesswitch/internal/session"
)

func TestLinePrioritizesTitleAndGroupsMetadata(t *testing.T) {
	item := session.Session{
		Provider: "codex", Title: "调研微信通知与控制方案", CWD: "/Users/sky/repos/home-lab",
		State: session.State{Kind: "needs_approval"}, Location: &session.Location{Kind: "tmux"},
	}
	got := New(item).Line()
	want := "🟠  ⬡  调研微信通知与控制方案  —  home-lab  ·  Codex › tmux  ·  Action needed"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestDoneOverridesRuntimeState(t *testing.T) {
	item := session.Session{
		Provider: "claude", Title: "Finished task", Source: "vscode", State: session.State{Kind: "working"},
		Task: &session.Task{Kind: "done"},
	}
	got := New(item)
	if got.StatusIcon != "✅" || got.ProviderIcon != "✦" || got.Status != "Done" || got.Destination != "VS Code" {
		t.Fatalf("unexpected entry: %+v", got)
	}
}

func TestStatePalette(t *testing.T) {
	wants := map[string]string{
		"needs_approval": "🟠", "working": "🔵", "turn_ended": "🟢",
		"session_open": "🟣", "interrupted": "🔴", "closed": "⚫",
		"error": "🔴", "idle": "🟡", "saved": "⚪",
	}
	for kind, want := range wants {
		got := New(session.Session{State: session.State{Kind: kind}})
		if got.StatusIcon != want {
			t.Errorf("%s: got %q, want %q", kind, got.StatusIcon, want)
		}
	}
}

func TestProviderIconsAreDistinct(t *testing.T) {
	codex := New(session.Session{Provider: "codex"})
	claude := New(session.Session{Provider: "claude"})
	if codex.ProviderIcon != "⬡" || claude.ProviderIcon != "✦" || codex.ProviderIcon == claude.ProviderIcon {
		t.Fatalf("unexpected provider icons: codex=%q claude=%q", codex.ProviderIcon, claude.ProviderIcon)
	}
}
