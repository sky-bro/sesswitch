package codex

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/sky-bro/sesswitch/internal/session"
)

type rpcMessage struct {
	ID     int             `json:"id,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type threadList struct {
	Data       []thread `json:"data"`
	NextCursor *string  `json:"nextCursor"`
}

type thread struct {
	ID         string          `json:"id"`
	SessionID  string          `json:"sessionId"`
	Name       *string         `json:"name"`
	Preview    string          `json:"preview"`
	CWD        string          `json:"cwd"`
	UpdatedAt  int64           `json:"updatedAt"`
	Status     json.RawMessage `json:"status"`
	Source     json.RawMessage `json:"source"`
	Originator *string         `json:"originator"`
}

type Adapter struct{}

func New() Adapter { return Adapter{} }

func (Adapter) Name() string { return "codex" }

func (Adapter) List(ctx context.Context, limit int) ([]session.Session, error) {
	return List(ctx, limit)
}

func executable() string {
	name := os.Getenv("SESSWITCH_CODEX")
	if name == "" {
		name = os.Getenv("AGENT_LOCATOR_CODEX") // pre-rename compatibility
	}
	if name == "" {
		name = "codex"
	}
	return name
}

func List(ctx context.Context, limit int) ([]session.Session, error) {
	if limit < 1 {
		return nil, fmt.Errorf("limit must be positive")
	}
	cmd := exec.CommandContext(ctx, executable(), "app-server", "--listen", "stdio://")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start Codex app-server: %w", err)
	}
	defer func() {
		_ = stdin.Close()
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		_ = cmd.Wait()
	}()

	encoder := json.NewEncoder(stdin)
	if err := encoder.Encode(map[string]any{
		"method": "initialize",
		"id":     1,
		"params": map[string]any{"clientInfo": map[string]string{
			"name": "sesswitch", "title": "Sesswitch", "version": "0.1.0",
		}},
	}); err != nil {
		return nil, err
	}

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 64*1024), 8*1024*1024)
	var threads []thread
	requestID := 2
	cursor := ""
	requestPage := func() error {
		pageSize := limit - len(threads)
		if pageSize > 100 {
			pageSize = 100
		}
		params := map[string]any{"limit": pageSize, "sortKey": "updated_at", "sortDirection": "desc"}
		if cursor != "" {
			params["cursor"] = cursor
		}
		return encoder.Encode(map[string]any{"method": "thread/list", "id": requestID, "params": params})
	}
	for scanner.Scan() {
		var message rpcMessage
		if err := json.Unmarshal(scanner.Bytes(), &message); err != nil {
			continue
		}
		if message.ID == 1 {
			if message.Error != nil {
				return nil, fmt.Errorf("initialize Codex app-server: %s", message.Error.Message)
			}
			if err := encoder.Encode(map[string]any{"method": "initialized", "params": map[string]any{}}); err != nil {
				return nil, err
			}
			if err := requestPage(); err != nil {
				return nil, err
			}
			continue
		}
		if message.ID != requestID {
			continue
		}
		if message.Error != nil {
			return nil, fmt.Errorf("list Codex threads: %s", message.Error.Message)
		}
		var result threadList
		if err := json.Unmarshal(message.Result, &result); err != nil {
			return nil, fmt.Errorf("decode Codex threads: %w", err)
		}
		threads = append(threads, result.Data...)
		if len(threads) >= limit || result.NextCursor == nil || *result.NextCursor == "" || *result.NextCursor == cursor || len(result.Data) == 0 {
			if len(threads) > limit {
				threads = threads[:limit]
			}
			return convert(threads), nil
		}
		cursor = *result.NextCursor
		requestID++
		if err := requestPage(); err != nil {
			return nil, err
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	message := strings.TrimSpace(stderr.String())
	if message == "" {
		message = "app-server closed without a response"
	}
	return nil, fmt.Errorf("list Codex threads: %s", message)
}

// Rename sets the user-facing name of a persisted Codex thread through the
// stable app-server API. It deliberately avoids editing rollout files.
func Rename(ctx context.Context, threadID, name string) error {
	threadID = strings.TrimSpace(threadID)
	name = strings.TrimSpace(name)
	if threadID == "" {
		return errors.New("thread ID must not be empty")
	}
	if name == "" {
		return errors.New("thread name must not be empty")
	}

	cmd := exec.CommandContext(ctx, executable(), "app-server", "--listen", "stdio://")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start Codex app-server: %w", err)
	}
	defer func() {
		_ = stdin.Close()
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		_ = cmd.Wait()
	}()

	encoder := json.NewEncoder(stdin)
	if err := encoder.Encode(map[string]any{
		"method": "initialize",
		"id":     1,
		"params": map[string]any{"clientInfo": map[string]string{
			"name": "sesswitch", "title": "Sesswitch", "version": "0.1.0",
		}},
	}); err != nil {
		return err
	}

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 64*1024), 8*1024*1024)
	for scanner.Scan() {
		var message rpcMessage
		if err := json.Unmarshal(scanner.Bytes(), &message); err != nil {
			continue
		}
		switch message.ID {
		case 1:
			if message.Error != nil {
				return fmt.Errorf("initialize Codex app-server: %s", message.Error.Message)
			}
			if err := encoder.Encode(map[string]any{"method": "initialized", "params": map[string]any{}}); err != nil {
				return err
			}
			if err := encoder.Encode(map[string]any{
				"method": "thread/name/set",
				"id":     2,
				"params": map[string]string{"threadId": threadID, "name": name},
			}); err != nil {
				return err
			}
		case 2:
			if message.Error != nil {
				return fmt.Errorf("rename Codex thread: %s", message.Error.Message)
			}
			return nil
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	message := strings.TrimSpace(stderr.String())
	if message == "" {
		message = "app-server closed without a response"
	}
	return fmt.Errorf("rename Codex thread: %s", message)
}

func convert(threads []thread) []session.Session {
	result := make([]session.Session, 0, len(threads))
	for _, item := range threads {
		title := ""
		if item.Name != nil {
			title = strings.TrimSpace(*item.Name)
		}
		if title == "" {
			title = firstLine(item.Preview)
		}
		if title == "" {
			title = filepath.Base(item.CWD)
		}
		if title == "" || title == "." || title == string(filepath.Separator) {
			title = "Codex session"
		}
		status, flags := threadStatus(item.Status)
		result = append(result, session.Session{
			Key:         "codex:" + item.ID,
			Provider:    "codex",
			ID:          item.ID,
			SessionID:   item.SessionID,
			Title:       title,
			CWD:         item.CWD,
			UpdatedAt:   time.Unix(item.UpdatedAt, 0).UTC(),
			Status:      status,
			ActiveFlags: flags,
			Source:      sourceName(item.Source, item.Originator),
		})
	}
	return result
}

func firstLine(value string) string {
	value = strings.Join(strings.Fields(strings.Split(value, "\n")[0]), " ")
	const max = 100
	if len([]rune(value)) > max {
		return string([]rune(value)[:max-1]) + "…"
	}
	return value
}

func threadStatus(raw json.RawMessage) (string, []string) {
	var value struct {
		Type        string   `json:"type"`
		ActiveFlags []string `json:"activeFlags"`
	}
	if json.Unmarshal(raw, &value) == nil && value.Type != "" {
		return value.Type, value.ActiveFlags
	}
	return "unknown", nil
}

func sourceName(raw json.RawMessage, originator *string) string {
	var text string
	if json.Unmarshal(raw, &text) == nil && text != "" {
		return text
	}
	var value map[string]any
	if json.Unmarshal(raw, &value) == nil {
		for key := range value {
			return key
		}
	}
	if originator != nil && *originator != "" {
		return *originator
	}
	return "unknown"
}
