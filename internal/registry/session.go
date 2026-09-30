package registry

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/sky-bro/sesswitch/internal/session"
)

// PutSession records only normalized metadata supplied by a documented
// provider interface. It deliberately does not retain prompts or transcripts.
func (s *Store) PutSession(item session.Session) error {
	key, err := Key(item.Provider, item.ID)
	if err != nil {
		return err
	}
	path := filepath.Join(s.sessionDir, key+".json")
	if existingData, readErr := os.ReadFile(path); readErr == nil {
		var existing session.Session
		if json.Unmarshal(existingData, &existing) == nil {
			if item.Title == "" {
				item.Title = existing.Title
			}
			if item.CWD == "" {
				item.CWD = existing.CWD
			}
		}
	}
	if item.Title == "" {
		item.Title = filepath.Base(item.CWD)
	}
	if item.Title == "" || item.Title == "." || item.Title == string(filepath.Separator) {
		item.Title = item.Provider + " session"
	}
	item.Key = item.Provider + ":" + item.ID
	item.SessionID = item.ID
	item.Location = nil
	item.Activity = nil
	item.Task = nil
	item.State = session.State{}
	if item.UpdatedAt.IsZero() {
		item.UpdatedAt = time.Now().UTC()
	}
	data, err := json.Marshal(item)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(s.sessionDir, ".session-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

func (s *Store) Sessions(provider string) ([]session.Session, error) {
	if !safeID.MatchString(provider) {
		return nil, fmt.Errorf("invalid provider %q", provider)
	}
	entries, err := os.ReadDir(s.sessionDir)
	if err != nil {
		return nil, err
	}
	items := make([]session.Session, 0)
	prefix := provider + "--"
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" || len(entry.Name()) <= len(prefix)+len(".json") || entry.Name()[:len(prefix)] != prefix {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.sessionDir, entry.Name()))
		if err != nil {
			continue
		}
		var item session.Session
		if json.Unmarshal(data, &item) != nil || item.Provider != provider || item.ID == "" {
			continue
		}
		items = append(items, item)
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].UpdatedAt.After(items[j].UpdatedAt) })
	return items, nil
}
