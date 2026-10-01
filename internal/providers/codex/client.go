package codex

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
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

type threadRead struct {
	Thread thread `json:"thread"`
}

type threadInput struct {
	Type string `json:"type"`
	Text string `json:"text"`
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
	Path       string          `json:"path"`
}

type Adapter struct{}

func New() Adapter { return Adapter{} }

func (Adapter) Name() string { return "codex" }

func (Adapter) List(ctx context.Context, limit int) ([]session.Session, error) {
	return List(ctx, limit)
}

func List(ctx context.Context, limit int) ([]session.Session, error) {
	if limit < 1 {
		return nil, fmt.Errorf("limit must be positive")
	}
	rpc, err := startAppServer(ctx)
	if err != nil {
		return nil, err
	}
	defer rpc.close()
	encoder, scanner := rpc.encoder, rpc.scanner
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
	message := rpc.endMessage()
	return nil, fmt.Errorf("list Codex threads: %s", message)
}

// Read fetches one Codex thread without scanning the full thread catalog.
func Read(ctx context.Context, threadID string) (session.Session, error) {
	threadID = strings.TrimSpace(threadID)
	if threadID == "" {
		return session.Session{}, errors.New("thread ID must not be empty")
	}
	rpc, err := startAppServer(ctx)
	if err != nil {
		return session.Session{}, err
	}
	defer rpc.close()
	encoder, scanner := rpc.encoder, rpc.scanner
	for scanner.Scan() {
		var message rpcMessage
		if err := json.Unmarshal(scanner.Bytes(), &message); err != nil {
			continue
		}
		switch message.ID {
		case 1:
			if message.Error != nil {
				return session.Session{}, fmt.Errorf("initialize Codex app-server: %s", message.Error.Message)
			}
			if err := encoder.Encode(map[string]any{"method": "initialized", "params": map[string]any{}}); err != nil {
				return session.Session{}, err
			}
			if err := encoder.Encode(map[string]any{
				"method": "thread/read",
				"id":     2,
				"params": map[string]any{"threadId": threadID, "includeTurns": false},
			}); err != nil {
				return session.Session{}, err
			}
		case 2:
			if message.Error != nil {
				return session.Session{}, fmt.Errorf("read Codex thread: %s", message.Error.Message)
			}
			var result threadRead
			if err := json.Unmarshal(message.Result, &result); err != nil {
				return session.Session{}, fmt.Errorf("decode Codex thread: %w", err)
			}
			items := convert([]thread{result.Thread})
			if len(items) != 1 || items[0].ID != threadID {
				return session.Session{}, errors.New("read Codex thread: response has no thread")
			}
			item := items[0]
			if isBrowserSource(item.Source) {
				item.BrowserTabID, item.BrowserURL, _ = browserTabFromRollout(ctx, result.Thread.Path)
			}
			return item, nil
		}
	}
	if err := scanner.Err(); err != nil {
		return session.Session{}, err
	}
	message := rpc.endMessage()
	return session.Session{}, fmt.Errorf("read Codex thread: %s", message)
}

type rolloutRecord struct {
	Type    string `json:"type"`
	Payload struct {
		Type    string        `json:"type"`
		Role    string        `json:"role"`
		Content []threadInput `json:"content"`
	} `json:"payload"`
}

func browserTabFromRollout(ctx context.Context, path string) (string, string, error) {
	if strings.TrimSpace(path) == "" {
		return "", "", errors.New("thread has no rollout path")
	}
	file, err := os.Open(path)
	if err != nil {
		return "", "", err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 8*1024*1024)
	var latestID, latestURL string
	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return "", "", err
		}
		line := scanner.Bytes()
		if !bytes.Contains(line, []byte("[selected] Tab ID")) {
			continue
		}
		var record rolloutRecord
		if json.Unmarshal(line, &record) != nil || record.Type != "response_item" || record.Payload.Type != "message" || record.Payload.Role != "user" {
			continue
		}
		for _, content := range record.Payload.Content {
			if content.Type != "input_text" && content.Type != "text" {
				continue
			}
			if !strings.Contains(content.Text, "# Chrome tabs:") {
				continue
			}
			matches := selectedChromeTabPattern.FindAllStringSubmatch(content.Text, -1)
			if len(matches) == 1 {
				latestID, latestURL = matches[0][1], matches[0][2]
			} else {
				latestID, latestURL = "", ""
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return "", "", err
	}
	return latestID, latestURL, nil
}

var selectedChromeTabPattern = regexp.MustCompile(`(?m)^\s*-\s*\[selected\]\s*Tab ID\s+([0-9]+):\s*(\S+)\s*$`)

func isBrowserSource(source string) bool {
	source = strings.ToLower(source)
	return strings.Contains(source, "chrome") || strings.Contains(source, "browser")
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

	rpc, err := startAppServer(ctx)
	if err != nil {
		return err
	}
	defer rpc.close()
	encoder, scanner := rpc.encoder, rpc.scanner
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
	message := rpc.endMessage()
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
	// Codex browser and desktop clients currently use the generic "vscode"
	// source value. Their originator is the more precise host identifier.
	if originator != nil {
		name := strings.TrimSpace(*originator)
		lower := strings.ToLower(name)
		switch {
		case strings.Contains(lower, "chrome"),
			strings.Contains(lower, "browser"),
			strings.Contains(lower, "safari"),
			strings.Contains(lower, "edge"),
			strings.Contains(lower, "desktop"):
			return name
		}
	}
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
