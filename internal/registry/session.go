package registry

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/sky-bro/sesswitch/internal/session"
)

// PutSession records only normalized metadata supplied by a documented
// provider interface. It deliberately does not retain prompts or transcripts.
func (s *Store) PutSession(item session.Session) error {
	unlock, err := s.lockRecords()
	if err != nil {
		return err
	}
	defer unlock()
	return s.putSession(item)
}

func (s *Store) putSession(item session.Session) error {
	key, err := Key(item.Provider, item.ID)
	if err != nil {
		return err
	}
	path := filepath.Join(s.sessionDir, key+".json")
	if existingData, readErr := os.ReadFile(path); readErr == nil {
		var existing session.Session
		if json.Unmarshal(existingData, &existing) == nil {
			if strings.HasSuffix(item.Source, " hook") && item.Resume == "" {
				item.Resume = existing.Resume
			}
			if item.Host == "" {
				item.Host = existing.Host
			}
			if item.Title == "" {
				item.Title = existing.Title
			}
			if item.CWD == "" {
				item.CWD = existing.CWD
			}
			if strings.HasSuffix(item.Source, " hook") && existing.Source != "" {
				item.Source = existing.Source
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
	if item.SessionID == "" {
		item.SessionID = item.ID
	}
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
	if previous, err := os.ReadFile(path); err == nil && bytes.Equal(bytes.TrimSpace(previous), data) {
		return nil
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

// SetResume records a provider-confirmed limitation without changing activity.
func (s *Store) SetResume(provider, id, capability string) error {
	key, err := Key(provider, id)
	if err != nil {
		return err
	}
	unlock, err := s.lockRecords()
	if err != nil {
		return err
	}
	defer unlock()
	data, err := os.ReadFile(filepath.Join(s.sessionDir, key+".json"))
	if err != nil {
		return err
	}
	var item session.Session
	if err := json.Unmarshal(data, &item); err != nil {
		return err
	}
	item.Resume = capability
	return s.putSession(item)
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

// RememberHost stores provenance without retaining a stale pane or PID.
func (s *Store) RememberHost(provider, id, host string) error {
	key, err := Key(provider, id)
	if err != nil {
		return err
	}
	if host != "tmux" && host != "wezterm" {
		return nil
	}
	unlock, err := s.lockRecords()
	if err != nil {
		return err
	}
	defer unlock()
	data, err := os.ReadFile(filepath.Join(s.sessionDir, key+".json"))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var item session.Session
	if json.Unmarshal(data, &item) != nil || item.Provider != provider || item.ID != id {
		return nil
	}
	if item.Host == host {
		return nil
	}
	item.Host = host
	return s.putSession(item)
}
