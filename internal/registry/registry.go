package registry

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/sky-bro/sesswitch/internal/session"
)

var safeID = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

func Key(provider, id string) (string, error) {
	if !safeID.MatchString(provider) || !safeID.MatchString(id) {
		return "", fmt.Errorf("invalid provider/session id %q/%q", provider, id)
	}
	return provider + "--" + id, nil
}

type Store struct {
	dir         string
	activityDir string
	taskDir     string
	sessionDir  string
}

// WatchDirs are directories whose atomic record replacements affect snapshots.
func (s *Store) WatchDirs() []string {
	return []string{s.dir, s.activityDir, s.taskDir, s.sessionDir}
}

func New() (*Store, error) {
	stateHome := os.Getenv("XDG_STATE_HOME")
	if stateHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		stateHome = filepath.Join(home, ".local", "state")
	}
	base := filepath.Join(stateHome, "sesswitch")
	legacy := filepath.Join(stateHome, "agent-locator")
	if _, err := os.Stat(base); errors.Is(err, os.ErrNotExist) {
		if _, legacyErr := os.Stat(legacy); legacyErr == nil {
			if err := os.Rename(legacy, base); err != nil {
				return nil, fmt.Errorf("migrate legacy state: %w", err)
			}
		}
	}
	dir := filepath.Join(base, "locations")
	activityDir := filepath.Join(base, "activity")
	taskDir := filepath.Join(base, "tasks")
	sessionDir := filepath.Join(base, "sessions")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create registry: %w", err)
	}
	if err := os.MkdirAll(activityDir, 0o700); err != nil {
		return nil, fmt.Errorf("create activity registry: %w", err)
	}
	if err := os.MkdirAll(taskDir, 0o700); err != nil {
		return nil, fmt.Errorf("create task registry: %w", err)
	}
	if err := os.MkdirAll(sessionDir, 0o700); err != nil {
		return nil, fmt.Errorf("create session registry: %w", err)
	}
	return &Store{dir: dir, activityDir: activityDir, taskDir: taskDir, sessionDir: sessionDir}, nil
}

func (s *Store) path(id string) (string, error) {
	if !safeID.MatchString(id) {
		return "", fmt.Errorf("invalid session id %q", id)
	}
	return filepath.Join(s.dir, id+".json"), nil
}

func (s *Store) Put(id string, location session.Location) error {
	path, err := s.path(id)
	if err != nil {
		return err
	}
	if existing, found, err := s.Get(id); err != nil {
		return err
	} else if found {
		before, after := existing, location
		before.LastSeen, after.LastSeen = time.Time{}, time.Time{}
		before.NeedsAttention, after.NeedsAttention = false, false
		if before == after {
			return nil
		}
	}
	location.LastSeen = time.Now().UTC()
	data, err := json.Marshal(location)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(s.dir, ".location-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
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
	return os.Rename(tmpName, path)
}

func (s *Store) Delete(id string) error {
	path, err := s.path(id)
	if err != nil {
		return err
	}
	err = os.Remove(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

// Get reads one location without enumerating the registry. Invalid or partially
// written records are cache misses, as they are in List.
func (s *Store) Get(id string) (session.Location, bool, error) {
	path, err := s.path(id)
	if err != nil {
		return session.Location{}, false, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return session.Location{}, false, nil
	}
	if err != nil {
		return session.Location{}, false, err
	}
	var location session.Location
	if json.Unmarshal(data, &location) != nil {
		return session.Location{}, false, nil
	}
	return location, true, nil
}

func (s *Store) List() (map[string]session.Location, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, err
	}
	locations := make(map[string]session.Location)
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.dir, entry.Name()))
		if err != nil {
			continue
		}
		var location session.Location
		if json.Unmarshal(data, &location) != nil {
			continue
		}
		id := entry.Name()[:len(entry.Name())-len(".json")]
		locations[id] = location
	}
	return locations, nil
}
