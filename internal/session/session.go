package session

import "time"

type Location struct {
	Provider    string `json:"provider,omitempty"`
	Kind        string `json:"kind"`
	TmuxPane    string `json:"tmux_pane,omitempty"`
	WezTermPane string `json:"wezterm_pane,omitempty"`
	TTY         string `json:"tty,omitempty"`
	AgentPID    int    `json:"agent_pid,omitempty"`
	AgentStart  string `json:"agent_start,omitempty"`
	// NeedsAttention is derived from the current host UI and must never be
	// persisted as part of a reusable location lease.
	NeedsAttention bool      `json:"-"`
	LastSeen       time.Time `json:"last_seen"`
}

// Activity describes an observed agent event, not the completion of a task.
type Activity struct {
	Provider   string    `json:"provider,omitempty"`
	Kind       string    `json:"kind"`
	Source     string    `json:"source"`
	ObservedAt time.Time `json:"observed_at"`
	TurnID     string    `json:"turn_id,omitempty"`
	AgentPID   int       `json:"agent_pid,omitempty"`
	AgentStart string    `json:"agent_start,omitempty"`
}

type State struct {
	Kind       string     `json:"kind"`
	Source     string     `json:"source"`
	ObservedAt *time.Time `json:"observed_at,omitempty"`
}

// Task is an explicit user judgment, independent of agent runtime events.
type Task struct {
	Kind      string    `json:"kind"`
	Source    string    `json:"source"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Session struct {
	Key          string    `json:"key"`
	Provider     string    `json:"provider"`
	ID           string    `json:"id"`
	SessionID    string    `json:"session_id,omitempty"`
	Title        string    `json:"title"`
	CWD          string    `json:"cwd,omitempty"`
	UpdatedAt    time.Time `json:"updated_at"`
	Status       string    `json:"status"`
	ActiveFlags  []string  `json:"active_flags,omitempty"`
	Source       string    `json:"source,omitempty"`
	BrowserTabID string    `json:"-"`
	BrowserURL   string    `json:"-"`
	Location     *Location `json:"location,omitempty"`
	Activity     *Activity `json:"activity,omitempty"`
	State        State     `json:"state"`
	Task         *Task     `json:"task,omitempty"`
}
