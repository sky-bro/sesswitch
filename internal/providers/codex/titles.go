package codex

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/sky-bro/sesswitch/internal/session"
)

// AddTitleHistory reads optional local name metadata, never transcripts. The
// index is an unstable, append-only Codex format; absent/incomplete metadata
// leaves ordinary current-title matching intact. A history is accepted only
// when its latest name agrees with the provider-confirmed catalog title.
func AddTitleHistory(ctx context.Context, items []session.Session) {
	wanted := make(map[string]bool)
	for i := range items {
		if items[i].Provider == "codex" {
			items[i].TitleAliases = nil
			wanted[items[i].ID] = true
		}
	}
	if len(wanted) == 0 {
		return
	}
	home := os.Getenv("CODEX_HOME")
	if home == "" {
		userHome, err := os.UserHomeDir()
		if err != nil {
			return
		}
		home = filepath.Join(userHome, ".codex")
	}
	file, err := os.Open(filepath.Join(home, "session_index.jsonl"))
	if err != nil {
		return
	}
	defer file.Close()
	history := make(map[string][]string)
	latest := make(map[string]string)
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		if ctx.Err() != nil {
			return
		}
		var record struct {
			ID   string `json:"id"`
			Name string `json:"thread_name"`
		}
		if json.Unmarshal(scanner.Bytes(), &record) != nil || !wanted[record.ID] {
			continue
		}
		name := strings.TrimSpace(record.Name)
		latest[record.ID] = name
		if name != "" && !slices.Contains(history[record.ID], name) {
			history[record.ID] = append(history[record.ID], name)
		}
	}
	if scanner.Err() != nil || ctx.Err() != nil {
		return
	}
	for i := range items {
		item := &items[i]
		if item.Provider != "codex" || latest[item.ID] != strings.TrimSpace(item.Title) {
			continue
		}
		for _, name := range history[item.ID] {
			if name != strings.TrimSpace(item.Title) {
				item.TitleAliases = append(item.TitleAliases, name)
			}
		}
	}
}
