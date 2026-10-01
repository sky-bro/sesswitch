// Package toolenv resolves tools and constructs one execution environment for
// providers, hosts, and launchers. It never loads shell startup files.
package toolenv

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var ErrConfiguration = errors.New("invalid tool configuration")

var names = []string{"node", "codex", "claude", "tmux", "wezterm", "vicinae", "ps", "open", "osascript", "env"}

type Selection struct{ Path, Source string }
type config struct {
	Tools map[string]string `json:"tools"`
}

type Resolver struct {
	environment []string
	selected    map[string]Selection
	failures    map[string]error
	configErr   error
}

type contextKey struct{}

// Context captures configuration and the inherited environment once per command.
func Context(ctx context.Context) context.Context {
	if _, ok := ctx.Value(contextKey{}).(*Resolver); ok {
		return ctx
	}
	return WithResolver(ctx, New())
}
func WithResolver(ctx context.Context, resolver *Resolver) context.Context {
	return context.WithValue(ctx, contextKey{}, resolver)
}
func FromContext(ctx context.Context) *Resolver {
	if resolver, ok := ctx.Value(contextKey{}).(*Resolver); ok {
		return resolver
	}
	return New()
}

func New() *Resolver {
	environment := os.Environ()
	home, err := os.UserHomeDir()
	if err != nil {
		return &Resolver{configErr: err}
	}
	settings, err := readConfig(environment, home)
	resolver := newResolver(environment, home, standardDirs(home), settings)
	resolver.configErr = err
	return resolver
}

func readConfig(environment []string, home string) (config, error) {
	path := envValue(environment, "SESSWITCH_CONFIG")
	explicit := path != ""
	if !explicit {
		root := envValue(environment, "XDG_CONFIG_HOME")
		if root == "" {
			root = filepath.Join(home, ".config")
		}
		path = filepath.Join(root, "sesswitch", "config.json")
	}
	path = expandHome(path, home)
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) && !explicit {
		return config{}, nil
	}
	if err != nil {
		return config{}, fmt.Errorf("read tool config %s: %w", path, err)
	}
	var settings config
	if err := json.Unmarshal(data, &settings); err != nil {
		return config{}, fmt.Errorf("decode tool config %s: %w", path, err)
	}
	for name := range settings.Tools {
		found := false
		for _, supported := range names {
			if name == supported {
				found = true
				break
			}
		}
		if !found {
			return config{}, fmt.Errorf("tool config %s: unsupported tool %q", path, name)
		}
	}
	return settings, nil
}

func newResolver(environment []string, home string, fallback []string, settings config) *Resolver {
	resolver := &Resolver{selected: map[string]Selection{}, failures: map[string]error{}}
	inherited := cleanDirs(filepath.SplitList(envValue(environment, "PATH")))
	fallback = cleanDirs(fallback)
	for _, name := range names {
		override := strings.TrimSpace(envValue(environment, "SESSWITCH_"+strings.ToUpper(name)))
		source := "environment"
		if override == "" && name == "codex" {
			override = strings.TrimSpace(envValue(environment, "AGENT_LOCATOR_CODEX"))
		}
		if override == "" {
			override = strings.TrimSpace(settings.Tools[name])
			source = "config"
		}
		var path string
		var err error
		if override != "" {
			override = expandHome(override, home)
			if strings.ContainsRune(override, filepath.Separator) {
				path, err = executablePath(override)
			} else {
				path, err = search(override, append(append([]string{}, inherited...), fallback...))
			}
			if err != nil {
				err = fmt.Errorf("%w: %s %q: %w", ErrConfiguration, name, override, err)
			}
		} else {
			source = "PATH"
			path, err = search(name, inherited)
			if err != nil {
				source = "standard"
				path, err = search(name, fallback)
			}
		}
		if err != nil {
			resolver.failures[name] = err
		} else {
			resolver.selected[name] = Selection{Path: path, Source: source}
		}
	}
	// The selected Node must precede other installations for env-node scripts and
	// child processes. All directly invoked tools still use their absolute paths.
	dirs := []string{}
	for _, name := range names {
		if selection, ok := resolver.selected[name]; ok {
			dirs = append(dirs, filepath.Dir(selection.Path))
		}
	}
	dirs = cleanDirs(append(append(dirs, inherited...), fallback...))
	resolver.environment = replaceEnv(environment, "PATH", strings.Join(dirs, string(os.PathListSeparator)))
	return resolver
}

func (resolver *Resolver) Resolve(name string) (Selection, error) {
	if resolver.configErr != nil {
		return Selection{}, fmt.Errorf("%w: %v", ErrConfiguration, resolver.configErr)
	}
	if selection, ok := resolver.selected[name]; ok {
		return selection, nil
	}
	if err, ok := resolver.failures[name]; ok {
		return Selection{}, fmt.Errorf("%s: %w", name, err)
	}
	return Selection{}, fmt.Errorf("unsupported tool %q", name)
}
func (resolver *Resolver) Environment() []string { return append([]string{}, resolver.environment...) }

// invocation also resolves a Node interpreter for npm agent entrypoints. This
// supports explicitly named Node binaries even when their basename is not node.
func (resolver *Resolver) invocation(name string, args []string) (string, []string, error) {
	selected, err := resolver.Resolve(name)
	if err != nil {
		return "", nil, err
	}
	if name == "codex" || name == "claude" {
		flags, needsNode, err := nodeShebang(selected.Path)
		if err != nil {
			return "", nil, fmt.Errorf("inspect %s: %w", selected.Path, err)
		}
		if needsNode {
			node, err := resolver.Resolve("node")
			if err != nil {
				return "", nil, fmt.Errorf("%s (%s) requires Node: %w", name, selected.Path, err)
			}
			argv := append(append(append([]string{}, flags...), selected.Path), args...)
			return node.Path, argv, nil
		}
	}
	return selected.Path, append([]string{}, args...), nil
}
func (resolver *Resolver) Check(name string) error {
	_, _, err := resolver.invocation(name, nil)
	return err
}

// Command uses the same environment for direct commands and processes started
// by an already-running WezTerm GUI. The latter cannot rely on cmd.Env alone.
func (resolver *Resolver) Command(ctx context.Context, name string, args ...string) (*exec.Cmd, error) {
	argv := append([]string{}, args...)
	if name == "wezterm" && len(argv) > 0 && (argv[0] == "start" || (len(argv) > 1 && argv[0] == "cli" && argv[1] == "spawn")) {
		for index, arg := range argv {
			if arg != "--" || index+1 >= len(argv) {
				continue
			}
			nested, nestedArgs, err := resolver.invocation(argv[index+1], argv[index+2:])
			if err != nil {
				return nil, err
			}
			env, err := resolver.Resolve("env")
			if err != nil {
				return nil, err
			}
			child := append([]string{env.Path}, resolver.childEnvironment()...)
			child = append(child, nested)
			child = append(child, nestedArgs...)
			argv = append(argv[:index+1], child...)
			break
		}
	}
	path, argv, err := resolver.invocation(name, argv)
	if err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(ctx, path, argv...)
	cmd.Env = resolver.Environment()
	return cmd, nil
}

// Forward tool/configuration state through the GUI without replacing its
// terminal-specific variables or placing credentials in process arguments.
func (resolver *Resolver) childEnvironment() []string {
	keys := []string{"HOME", "XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_STATE_HOME", "CODEX_HOME", "CLAUDE_CONFIG_DIR", "SESSWITCH_CONFIG", "AGENT_LOCATOR_CODEX"}
	for _, name := range names {
		keys = append(keys, "SESSWITCH_"+strings.ToUpper(name))
	}
	args := []string{}
	assignments := []string{"PATH=" + envValue(resolver.environment, "PATH")}
	for _, key := range keys {
		value := envValue(resolver.environment, key)
		if value == "" {
			args = append(args, "-u", key)
		} else {
			assignments = append(assignments, key+"="+value)
		}
	}
	return append(args, assignments...)
}

func Command(ctx context.Context, name string, args ...string) (*exec.Cmd, error) {
	return FromContext(ctx).Command(ctx, name, args...)
}

func executablePath(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(absolute)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0111 == 0 {
		return "", fmt.Errorf("not executable: %s", absolute)
	}
	return absolute, nil
}
func search(name string, dirs []string) (string, error) {
	for _, dir := range dirs {
		if path, err := executablePath(filepath.Join(dir, name)); err == nil {
			return path, nil
		}
	}
	return "", exec.ErrNotFound
}
func cleanDirs(dirs []string) []string {
	result := []string{}
	seen := map[string]bool{}
	for _, dir := range dirs {
		// Relative and empty PATH components would make GUI selection depend on cwd.
		if !filepath.IsAbs(dir) {
			continue
		}
		dir = filepath.Clean(dir)
		if !seen[dir] {
			seen[dir] = true
			result = append(result, dir)
		}
	}
	return result
}
func expandHome(path, home string) string {
	if path == "~" {
		return home
	}
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(home, path[2:])
	}
	return path
}
func envValue(environment []string, key string) string {
	for index := len(environment) - 1; index >= 0; index-- {
		if value, ok := strings.CutPrefix(environment[index], key+"="); ok {
			return value
		}
	}
	return ""
}
func replaceEnv(environment []string, key, value string) []string {
	result := make([]string, 0, len(environment)+1)
	for _, entry := range environment {
		if !strings.HasPrefix(entry, key+"=") {
			result = append(result, entry)
		}
	}
	return append(result, key+"="+value)
}
func nodeShebang(path string) ([]string, bool, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, false, err
	}
	defer file.Close()
	line, _ := bufio.NewReader(io.LimitReader(file, 512)).ReadString('\n')
	if !strings.HasPrefix(line, "#!") {
		return nil, false, nil
	}
	fields := strings.Fields(strings.TrimPrefix(line, "#!"))
	if len(fields) == 0 {
		return nil, false, nil
	}
	if filepath.Base(fields[0]) == "env" {
		fields = fields[1:]
		if len(fields) > 0 && fields[0] == "-S" {
			fields = fields[1:]
		}
	}
	if len(fields) > 0 && filepath.Base(fields[0]) == "node" {
		return fields[1:], true, nil
	}
	return nil, false, nil
}
