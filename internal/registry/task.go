package registry

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/sky-bro/sesswitch/internal/session"
)

func (s *Store) PutTask(id string, task session.Task) error {
	if !safeID.MatchString(id) {
		return fmt.Errorf("invalid session id %q", id)
	}
	if task.Kind != "done" && task.Kind != "read" {
		return fmt.Errorf("unsupported task state %q", task.Kind)
	}
	unlock, err := s.lockRecords()
	if err != nil {
		return err
	}
	defer unlock()
	task.UpdatedAt = time.Now().UTC()
	data, err := json.Marshal(task)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(s.taskDir, ".task-*")
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
	return os.Rename(tmp.Name(), filepath.Join(s.taskDir, id+".json"))
}

func (s *Store) ClearTask(id string) error {
	if !safeID.MatchString(id) {
		return fmt.Errorf("invalid session id %q", id)
	}
	unlock, err := s.lockRecords()
	if err != nil {
		return err
	}
	defer unlock()
	err = os.Remove(filepath.Join(s.taskDir, id+".json"))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func (s *Store) Tasks() (map[string]session.Task, error) {
	entries, err := os.ReadDir(s.taskDir)
	if err != nil {
		return nil, err
	}
	tasks := make(map[string]session.Task)
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		id := entry.Name()[:len(entry.Name())-len(".json")]
		if !safeID.MatchString(id) {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.taskDir, entry.Name()))
		if err != nil {
			continue
		}
		var task session.Task
		if json.Unmarshal(data, &task) == nil && (task.Kind == "done" || task.Kind == "read") {
			tasks[id] = task
		}
	}
	return tasks, nil
}
