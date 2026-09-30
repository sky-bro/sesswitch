package registry

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/sky-bro/sesswitch/internal/session"
)

func (s *Store) PutActivity(id string, activity session.Activity) error {
	if !safeID.MatchString(id) {
		return fmt.Errorf("invalid session id %q", id)
	}
	activity.ObservedAt = time.Now().UTC()
	data, err := json.Marshal(activity)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(s.activityDir, ".activity-*")
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
	return os.Rename(tmp.Name(), filepath.Join(s.activityDir, id+".json"))
}

func (s *Store) Activities() (map[string]session.Activity, error) {
	entries, err := os.ReadDir(s.activityDir)
	if err != nil {
		return nil, err
	}
	activities := make(map[string]session.Activity)
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		id := entry.Name()[:len(entry.Name())-len(".json")]
		if !safeID.MatchString(id) {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.activityDir, entry.Name()))
		if err != nil {
			continue
		}
		var activity session.Activity
		if json.Unmarshal(data, &activity) == nil && activity.Kind != "" {
			activities[id] = activity
		}
	}
	return activities, nil
}
