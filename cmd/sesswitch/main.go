package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/sky-bro/sesswitch/internal/hosts"
	openagent "github.com/sky-bro/sesswitch/internal/open"
	"github.com/sky-bro/sesswitch/internal/presentation"
	"github.com/sky-bro/sesswitch/internal/process"
	"github.com/sky-bro/sesswitch/internal/providers"
	"github.com/sky-bro/sesswitch/internal/providers/claude"
	"github.com/sky-bro/sesswitch/internal/providers/codex"
	"github.com/sky-bro/sesswitch/internal/registry"
	"github.com/sky-bro/sesswitch/internal/session"
	"github.com/sky-bro/sesswitch/internal/state"
)

const version = "0.1.0"

func main() {
	if err := run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "sesswitch:", err)
		os.Exit(1)
	}
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		usage(stderr)
		return flag.ErrHelp
	}
	switch args[0] {
	case "list":
		return listCommand(args[1:], stdout)
	case "open":
		return openCommand(args[1:])
	case "pick":
		return pickCommand(args[1:])
	case "hook":
		return hookCommand(args[1:], stdin)
	case "mark":
		return markCommand(args[1:])
	case "rename":
		return renameCommand(args[1:])
	case "doctor":
		return doctorCommand(stdout)
	case "version", "--version", "-v":
		fmt.Fprintln(stdout, version)
		return nil
	case "help", "--help", "-h":
		usage(stdout)
		return nil
	default:
		usage(stderr)
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func usage(w io.Writer) {
	fmt.Fprintln(w, `Usage: sesswitch <command>

Commands:
  list [--json] [--limit N]       List indexed AI sessions
  open [--target auto|terminal|app] <provider:id>
                                  Focus or resume a session
  pick                            Select a session with Vicinae dmenu
  hook codex|claude               Consume a provider hook event from stdin
  mark done|clear <provider:id>   Mark a task done or clear the mark
  rename <provider:id> <name>     Rename a session when its provider supports it
  doctor                          Check local integrations
  version                         Print the version`)
}

func sessions(ctx context.Context, limit int) ([]session.Session, error) {
	items, err := providers.List(ctx, []providers.Adapter{codex.New(), claude.New()}, limit)
	if err != nil {
		return nil, err
	}
	store, err := registry.New()
	if err != nil {
		return nil, err
	}
	locations, err := store.List()
	if err != nil {
		return nil, err
	}
	activities, err := store.Activities()
	if err != nil {
		return nil, err
	}
	tasks, err := store.Tasks()
	if err != nil {
		return nil, err
	}
	for index := range items {
		registryID, keyErr := registry.Key(items[index].Provider, items[index].ID)
		if keyErr != nil {
			return nil, keyErr
		}
		location, ok := locations[registryID]
		if !ok { // pre-provider namespace compatibility
			location, ok = locations[items[index].ID]
		}
		if !ok && items[index].SessionID != "" {
			sessionRegistryID, _ := registry.Key(items[index].Provider, items[index].SessionID)
			location, ok = locations[sessionRegistryID]
			if !ok {
				location, ok = locations[items[index].SessionID]
			}
		}
		if ok {
			items[index].Location = &location
		}
		activity, found := activities[registryID]
		if !found {
			activity, found = activities[items[index].ID]
		}
		if !found && items[index].SessionID != "" {
			sessionRegistryID, _ := registry.Key(items[index].Provider, items[index].SessionID)
			activity, found = activities[sessionRegistryID]
			if !found {
				activity, found = activities[items[index].SessionID]
			}
		}
		if found {
			items[index].Activity = &activity
		}
		task, found := tasks[registryID]
		if !found {
			task, found = tasks[items[index].ID]
		}
		if !found && items[index].SessionID != "" {
			sessionRegistryID, _ := registry.Key(items[index].Provider, items[index].SessionID)
			task, found = tasks[sessionRegistryID]
			if !found {
				task, found = tasks[items[index].SessionID]
			}
		}
		if found {
			items[index].Task = &task
		}
		items[index].State = state.Resolve(ctx, items[index])
	}
	discovered := hosts.DiscoverTmuxLocations(ctx, items, openagent.CommandOutputRunner, process.AgentInfoAtTTY)
	for index := range items {
		if location, ok := discovered[items[index].Key]; ok {
			items[index].Location = &location
			items[index].State = state.Resolve(ctx, items[index])
		}
	}
	return items, nil
}

func listCommand(args []string, stdout io.Writer) error {
	flags := flag.NewFlagSet("list", flag.ContinueOnError)
	jsonOutput := flags.Bool("json", false, "write JSON")
	limit := flags.Int("limit", 100, "maximum number of sessions")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *limit < 1 || *limit > 1000 {
		return errors.New("limit must be between 1 and 1000")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	items, err := sessions(ctx, *limit)
	if err != nil {
		return err
	}
	if *jsonOutput {
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(items)
	}
	for _, item := range items {
		observed := "-"
		if item.State.ObservedAt != nil && !item.State.ObservedAt.IsZero() {
			observed = item.State.ObservedAt.Format(time.RFC3339)
		}
		task := "-"
		if item.Task != nil {
			task = item.Task.Kind
		}
		fmt.Fprintf(stdout, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n", item.Key, item.Title, item.CWD, item.State.Kind, item.State.Source, observed, task)
	}
	return nil
}

func openCommand(args []string) error {
	flags := flag.NewFlagSet("open", flag.ContinueOnError)
	target := flags.String("target", "auto", "auto, terminal, or app")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 1 {
		return errors.New("open requires one provider:id key")
	}
	return openKey(flags.Arg(0), *target)
}

func openKey(key, target string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	items, err := sessions(ctx, 1000)
	if err != nil {
		return err
	}
	for _, item := range items {
		if item.Key == key {
			return openagent.Session(context.Background(), item, target, openagent.CommandRunner)
		}
	}
	return fmt.Errorf("session %q was not found", key)
}

func pickCommand(args []string) error {
	flags := flag.NewFlagSet("pick", flag.ContinueOnError)
	target := flags.String("target", "auto", "auto, terminal, or app")
	if err := flags.Parse(args); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	items, err := sessions(ctx, 1000)
	if err != nil {
		return err
	}
	if len(items) == 0 {
		return errors.New("no sessions found")
	}
	var input bytes.Buffer
	for _, item := range items {
		fmt.Fprintln(&input, presentation.New(item).Line())
	}
	cmd := exec.Command("vicinae", "dmenu", "--format", "index", "--navigation-title", "AI Sessions", "--section-title", "Sessions ({count})", "--placeholder", "Search sessions, projects, or agents", "--width", "920", "--height", "640", "--no-metadata")
	cmd.Stdin = &input
	output, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return nil
		}
		return fmt.Errorf("Vicinae dmenu: %w", err)
	}
	index, err := strconv.Atoi(strings.TrimSpace(string(output)))
	if err != nil || index < 0 || index >= len(items) {
		return fmt.Errorf("invalid Vicinae selection %q", strings.TrimSpace(string(output)))
	}
	return openagent.Session(context.Background(), items[index], *target, openagent.CommandRunner)
}

func markCommand(args []string) error {
	if len(args) != 2 {
		return errors.New("mark requires done|clear and one provider:id key")
	}
	provider, id, ok := strings.Cut(args[1], ":")
	if !ok || provider == "" || id == "" {
		return fmt.Errorf("unsupported session key %q", args[1])
	}
	store, err := registry.New()
	if err != nil {
		return err
	}
	switch args[0] {
	case "done":
		registryID, err := registry.Key(provider, id)
		if err != nil {
			return err
		}
		return store.PutTask(registryID, session.Task{Kind: "done", Source: "user"})
	case "clear":
		registryID, err := registry.Key(provider, id)
		if err != nil {
			return err
		}
		return store.ClearTask(registryID)
	default:
		return fmt.Errorf("unknown task mark %q", args[0])
	}
}

func renameCommand(args []string) error {
	if len(args) != 2 {
		return errors.New("rename requires one provider:id key and a non-empty name")
	}
	provider, id, ok := strings.Cut(args[0], ":")
	if !ok || provider == "" || id == "" {
		return fmt.Errorf("unsupported session key %q", args[0])
	}
	if strings.TrimSpace(args[1]) == "" {
		return errors.New("session name must not be empty")
	}
	if provider != "codex" {
		return fmt.Errorf("rename is not supported for provider %q", provider)
	}
	key := args[0]
	lookupCtx, cancelLookup := context.WithTimeout(context.Background(), 12*time.Second)
	items, err := sessions(lookupCtx, 1000)
	cancelLookup()
	if err != nil {
		return fmt.Errorf("prepare rename: %w", err)
	}
	var target *session.Session
	for index := range items {
		if items[index].Key == key {
			target = &items[index]
			break
		}
	}
	if target == nil {
		return fmt.Errorf("session %q was not found", key)
	}
	if err := rememberLocation(*target); err != nil {
		return fmt.Errorf("preserve session location: %w", err)
	}

	renameCtx, cancelRename := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancelRename()
	return codex.Rename(renameCtx, id, args[1])
}

// rememberLocation freezes a freshly discovered live coordinate before a
// provider-side metadata change (such as rename) can invalidate title-based
// tmux discovery. Verified process identity still protects the stored lease.
func rememberLocation(item session.Session) error {
	if item.Location == nil {
		return nil
	}
	registryID, err := registry.Key(item.Provider, item.ID)
	if err != nil {
		return err
	}
	store, err := registry.New()
	if err != nil {
		return err
	}
	return store.Put(registryID, *item.Location)
}

type hookInput struct {
	SessionID      string `json:"session_id"`
	HookEventName  string `json:"hook_event_name"`
	TurnID         string `json:"turn_id"`
	Source         string `json:"source"`
	StopHookActive bool   `json:"stop_hook_active"`
	CWD            string `json:"cwd"`
	NewCWD         string `json:"new_cwd"`
	SessionTitle   string `json:"session_title"`
	Notification   string `json:"notification_type"`
}

func hookCommand(args []string, stdin io.Reader) error {
	if len(args) == 0 { // compatibility with the first Codex hook installer
		return providerHookCommand("codex", stdin)
	}
	if len(args) != 1 || (args[0] != "codex" && args[0] != "claude") {
		return errors.New("hook requires provider codex or claude")
	}
	return providerHookCommand(args[0], stdin)
}

func providerHookCommand(provider string, stdin io.Reader) error {
	var input hookInput
	decoder := json.NewDecoder(io.LimitReader(stdin, 1024*1024))
	if err := decoder.Decode(&input); err != nil {
		return fmt.Errorf("decode hook input: %w", err)
	}
	if input.SessionID == "" {
		return errors.New("hook input has no session_id")
	}
	registryID, err := registry.Key(provider, input.SessionID)
	if err != nil {
		return err
	}
	store, err := registry.New()
	if err != nil {
		return err
	}
	activityKind := hookActivityKind(input)
	location := session.Location{
		Provider:    provider,
		WezTermPane: os.Getenv("WEZTERM_PANE"),
	}
	if provider == "claude" {
		cwd := input.CWD
		if input.NewCWD != "" {
			cwd = input.NewCWD
		}
		title := strings.TrimSpace(input.SessionTitle)
		status := "observed"
		if input.HookEventName == "SessionEnd" {
			status = "closed"
		}
		if err := store.PutSession(session.Session{Provider: provider, ID: input.SessionID, Title: title, CWD: cwd, UpdatedAt: time.Now().UTC(), Status: status, Source: "Claude hook"}); err != nil {
			return err
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	agent, err := process.AgentAncestor(ctx, provider)
	if activityKind != "" {
		activity := session.Activity{Provider: provider, Kind: activityKind, Source: provider + "-hook:" + input.HookEventName, TurnID: input.TurnID}
		if err == nil {
			activity.AgentPID = agent.PID
			activity.AgentStart = agent.Start
		}
		if writeErr := store.PutActivity(registryID, activity); writeErr != nil {
			return writeErr
		}
	}
	if input.HookEventName == "SessionEnd" {
		if err := store.Delete(registryID); err != nil {
			return err
		}
		return store.Delete(input.SessionID)
	}
	if err != nil || agent.TTY == "" {
		// Do not register an unverified terminal coordinate from inherited env.
		return nil
	}
	location.AgentPID = agent.PID
	location.AgentStart = agent.Start
	location.TTY = agent.TTY
	if pane, err := tmuxPaneAtTTY(ctx, agent.TTY, openagent.CommandOutputRunner); err == nil {
		location.TmuxPane = pane
	}
	switch {
	case location.TmuxPane != "":
		location.Kind = "tmux"
	case location.WezTermPane != "":
		location.Kind = "wezterm"
	default:
		// GUI clients do not expose a stable, focusable host coordinate here.
		// Do not overwrite a useful terminal lease with an unfocusable process.
		return nil
	}
	return store.Put(registryID, location)
}

func hookActivityKind(input hookInput) string {
	switch input.HookEventName {
	case "SessionStart":
		if input.Source != "compact" {
			return "session_open"
		}
	case "UserPromptSubmit", "PostToolUse":
		return "working"
	case "PermissionRequest":
		return "needs_approval"
	case "Stop":
		if input.StopHookActive {
			return "working"
		}
		return "turn_ended"
	case "Interrupt":
		return "interrupted"
	case "StopFailure":
		return "interrupted"
	case "Notification":
		switch input.Notification {
		case "permission_prompt", "agent_needs_input":
			return "needs_approval"
		case "idle_prompt", "agent_completed":
			return "turn_ended"
		}
	case "SessionEnd":
		return "closed"
	}
	return ""
}

func tmuxPaneAtTTY(ctx context.Context, tty string, output openagent.OutputRunner) (string, error) {
	data, err := output(ctx, "tmux", "list-panes", "-a", "-F", "#{pane_tty}\t#{pane_id}")
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(data), "\n") {
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) == 2 && process.NormalizeTTY(parts[0]) == process.NormalizeTTY(tty) && strings.HasPrefix(parts[1], "%") {
			return parts[1], nil
		}
	}
	return "", fmt.Errorf("no tmux pane at %s", tty)
}

func doctorCommand(stdout io.Writer) error {
	failed := false
	for _, name := range []string{"tmux", "wezterm", "vicinae"} {
		path, err := exec.LookPath(name)
		if err != nil {
			fmt.Fprintf(stdout, "missing  %s\n", name)
			failed = true
		} else {
			fmt.Fprintf(stdout, "ok       %s (%s)\n", name, path)
		}
	}
	providersFound := 0
	for _, name := range []string{"codex", "claude"} {
		path, err := exec.LookPath(name)
		if err != nil {
			fmt.Fprintf(stdout, "optional %s (not installed)\n", name)
			continue
		}
		providersFound++
		fmt.Fprintf(stdout, "ok       %s (%s)\n", name, path)
	}
	if providersFound == 0 {
		fmt.Fprintln(stdout, "failed   no supported provider installed (codex or claude)")
		failed = true
	}
	if _, err := exec.LookPath("codex"); err == nil {
		ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
		defer cancel()
		items, listErr := codex.List(ctx, 1)
		if listErr != nil {
			fmt.Fprintf(stdout, "failed   Codex app-server (%v)\n", listErr)
			failed = true
		} else {
			fmt.Fprintf(stdout, "ok       Codex app-server (%d session sampled)\n", len(items))
		}
	}
	if failed {
		return errors.New("one or more checks failed")
	}
	return nil
}
