package registry

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/sky-bro/sesswitch/internal/session"
)

func (s *Store) PutActivity(id string, activity session.Activity) error {
	if !safeID.MatchString(id) {
		return fmt.Errorf("invalid session id %q", id)
	}
	if activity.ObservedAt.IsZero() {
		activity.ObservedAt = time.Now().UTC()
	}
	// Serialize hook writers; atomic rename alone cannot prevent an older
	// hook, delayed during process inspection, from overwriting a newer one.
	unlock, err := s.lockRecords()
	if err != nil {
		return err
	}
	defer unlock()
	if data, err := os.ReadFile(filepath.Join(s.activityDir, id+".json")); err == nil {
		var previous session.Activity
		if json.Unmarshal(data, &previous) == nil {
			if previous.AgentStart != "" && activity.AgentStart != "" && previous.AgentStart != activity.AgentStart {
				before, err1 := time.ParseInLocation("Mon Jan 2 15:04:05 2006", previous.AgentStart, time.Local)
				after, err2 := time.ParseInLocation("Mon Jan 2 15:04:05 2006", activity.AgentStart, time.Local)
				if err1 == nil && err2 == nil && after.Before(before) {
					return nil
				}
			}
			if !activity.ObservedAt.After(previous.ObservedAt) {
				return nil
			}
			if activity.TurnID != "" && activity.TurnID == previous.TurnID &&
				previous.Kind == "turn_ended" && strings.HasSuffix(activity.Source, ":PostToolUse") {
				return nil
			}
		}
	}
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
	if err := os.Rename(tmp.Name(), filepath.Join(s.activityDir, id+".json")); err != nil {
		return err
	}
	// Reopening is permanent: SessionEnd must not resurrect an older Done mark.
	if activity.Kind != "closed" {
		path := filepath.Join(s.taskDir, id+".json")
		data, err := os.ReadFile(path)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		var task session.Task
		if json.Unmarshal(data, &task) == nil && task.Kind == "done" && activity.ObservedAt.After(task.UpdatedAt) {
			if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
		}
	}
	return nil
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

// Share the hook lock with task writers so reopening cannot erase a newer mark.
func (s *Store) lockRecords() (func(), error) {
	lock, err := os.OpenFile(filepath.Join(s.activityDir, ".lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		lock.Close()
		return nil, err
	}
	return func() { syscall.Flock(int(lock.Fd()), syscall.LOCK_UN); lock.Close() }, nil
}
