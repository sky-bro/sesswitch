package toolenv

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestTmuxFormatOutputWithoutUTF8Locale(t *testing.T) {
	tmux, err := exec.LookPath("tmux")
	if err != nil {
		t.Skip("tmux is not installed")
	}
	// Use a private server so the regression check never touches user sessions.
	dir, err := os.MkdirTemp("", "ss-tmux-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	socket := filepath.Join(dir, "tmux.sock")
	environment := []string{"HOME=" + dir, "PATH=/usr/bin:/bin", "LANG=C", "LC_ALL=C"}
	start := exec.Command(tmux, "-S", socket, "-f", os.DevNull, "new-session", "-d", "-s", "gui", "/bin/sleep", "60")
	start.Env = environment
	if data, err := start.CombinedOutput(); err != nil {
		t.Fatalf("start private tmux server: %v: %s", err, data)
	}
	t.Cleanup(func() {
		stop := exec.Command(tmux, "-S", socket, "kill-server")
		stop.Env = environment
		_ = stop.Run()
	})
	resolver := newResolver(environment, dir, nil, config{Tools: map[string]string{"tmux": tmux}})
	cmd, err := resolver.Command(context.Background(), "tmux", "-S", socket, "display-message", "-p", "-t", "gui:0.0", "#{session_name}\t#{pane_tty}")
	if err != nil {
		t.Fatal(err)
	}
	data, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.SplitN(strings.TrimSpace(string(data)), "\t", 2)
	if len(parts) != 2 || parts[0] != "gui" || !strings.HasPrefix(parts[1], "/dev/") {
		t.Fatalf("tmux mangled format fields without a UTF-8 locale: %q", data)
	}
}

func writeTool(t *testing.T, dir, name, contents string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(contents), 0700); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestResolutionPrecedence(t *testing.T) {
	home := t.TempDir()
	explicit := writeTool(t, filepath.Join(home, "explicit"), "codex", "#!/bin/sh\n")
	configured := writeTool(t, filepath.Join(home, "configured"), "codex", "#!/bin/sh\n")
	inherited := writeTool(t, filepath.Join(home, "inherited"), "codex", "#!/bin/sh\n")
	fallback := writeTool(t, filepath.Join(home, "standard"), "codex", "#!/bin/sh\n")
	for _, test := range []struct{ name, override, config, path, want, source string }{
		{"environment", explicit, configured, filepath.Dir(inherited), explicit, "environment"},
		{"config", "", configured, filepath.Dir(inherited), configured, "config"},
		{"PATH", "", "", filepath.Dir(inherited), inherited, "PATH"},
		{"standard", "", "", "/absent", fallback, "standard"},
	} {
		t.Run(test.name, func(t *testing.T) {
			resolver := newResolver([]string{"PATH=" + test.path, "SESSWITCH_CODEX=" + test.override}, home, []string{filepath.Dir(fallback)}, config{Tools: map[string]string{"codex": test.config}})
			selected, err := resolver.Resolve("codex")
			if err != nil || selected.Path != test.want || selected.Source != test.source {
				t.Fatalf("selection=%+v err=%v", selected, err)
			}
		})
	}
	resolver := newResolver([]string{"PATH=" + filepath.Dir(inherited), "SESSWITCH_CODEX=" + filepath.Join(home, "missing")}, home, []string{filepath.Dir(fallback)}, config{})
	if _, err := resolver.Resolve("codex"); err == nil {
		t.Fatal("invalid explicit path silently fell back")
	}
}

func TestAllToolsAndLegacyOverride(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, "bin")
	for _, name := range names {
		writeTool(t, dir, name, "#!/bin/sh\n")
	}
	resolver := newResolver([]string{"PATH=/absent"}, home, []string{dir}, config{})
	for _, name := range names {
		selected, err := resolver.Resolve(name)
		if err != nil || selected.Path != filepath.Join(dir, name) {
			t.Fatalf("%s: %+v %v", name, selected, err)
		}
	}
	legacy := writeTool(t, filepath.Join(home, "legacy"), "custom-codex", "#!/bin/sh\n")
	resolver = newResolver([]string{"PATH=/absent", "AGENT_LOCATOR_CODEX=" + legacy}, home, []string{dir}, config{})
	selected, err := resolver.Resolve("codex")
	if err != nil || selected.Path != legacy {
		t.Fatalf("legacy: %+v %v", selected, err)
	}
}

func TestGUINpmAgentReceivesSelectedNodeAndEnvironment(t *testing.T) {
	home := t.TempDir()
	agent := writeTool(t, filepath.Join(home, "npm"), "codex", "#!/usr/bin/env node\nconsole.log('agent')\n")
	// A shell fixture impersonates Node, verifying argv and PATH without relying
	// on the developer machine's Node installation.
	node := writeTool(t, filepath.Join(home, "runtime with spaces"), "custom-node", "#!/bin/sh\nprintf '%s\\n' \"$1\" \"$PATH\" \"$OTHER\"\n")
	writeTool(t, filepath.Join(home, "other-node"), "node", "#!/bin/sh\nexit 99\n")
	resolver := newResolver([]string{"PATH=/usr/bin:/bin", "SESSWITCH_CODEX=" + agent, "SESSWITCH_NODE=" + node, "OTHER=kept"}, home, []string{filepath.Join(home, "other-node")}, config{})
	cmd, err := resolver.Command(context.Background(), "codex", "app-server")
	if err != nil {
		t.Fatal(err)
	}
	data, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 3 || lines[0] != agent || !strings.HasPrefix(lines[1], filepath.Dir(node)+string(os.PathListSeparator)) || lines[2] != "kept" {
		t.Fatalf("wrong execution: %q", data)
	}
	if cmd.Path != node || cmd.Args[2] != "app-server" {
		t.Fatalf("wrong interpreter: %q", cmd.Args)
	}
	selected, _ := resolver.Resolve("codex")
	if selected.Path != agent {
		t.Fatal("diagnosis disagrees with execution")
	}
	missing := newResolver([]string{"PATH=/absent", "SESSWITCH_CODEX=" + agent}, home, nil, config{})
	if err := missing.Check("codex"); err == nil || !strings.Contains(err.Error(), "requires Node") {
		t.Fatalf("missing Node undiagnosed: %v", err)
	}
	if _, err := missing.Command(context.Background(), "codex"); err == nil {
		t.Fatal("script without Node was accepted")
	}
}

func TestWezTermChildUsesAbsoluteToolAndExplicitPATH(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, "tools")
	for _, name := range []string{"wezterm", "codex", "claude", "tmux", "env"} {
		writeTool(t, dir, name, "#!/bin/sh\n")
	}
	resolver := newResolver([]string{"PATH=/usr/bin:/bin"}, home, []string{dir}, config{})
	for _, test := range []struct {
		args   []string
		nested string
	}{
		{[]string{"start", "--cwd", "/project with spaces", "--", "codex", "resume", "id"}, "codex"},
		{[]string{"start", "--", "claude", "--resume", "id"}, "claude"},
		{[]string{"cli", "spawn", "--window-id", "7", "--", "tmux", "attach-session", "-t", "work"}, "tmux"},
	} {
		cmd, err := resolver.Command(context.Background(), "wezterm", test.args...)
		if err != nil {
			t.Fatal(err)
		}
		envSelection, _ := resolver.Resolve("env")
		index := 0
		for cmd.Args[index] != "--" {
			index++
		}
		if cmd.Path != filepath.Join(dir, "wezterm") || cmd.Args[index+1] != envSelection.Path || (!containsArgument(cmd.Args, "PATH="+envValue(cmd.Env, "PATH"))) || (!containsArgument(cmd.Args, filepath.Join(dir, test.nested))) {
			t.Fatalf("wrong nested invocation: %q", cmd.Args)
		}
	}
	cmd, err := resolver.Command(context.Background(), "wezterm", "cli", "list", "--format", "json")
	if err != nil || !reflect.DeepEqual(cmd.Args[1:], []string{"cli", "list", "--format", "json"}) {
		t.Fatalf("non-spawn command rewritten: %v %v", cmd, err)
	}
}

func TestMultipleInstalledVersionsAreSortedNumerically(t *testing.T) {
	home := t.TempDir()
	for _, version := range []string{"v9.11.0", "v22.9.0", "v22.10.0"} {
		dir := filepath.Join(home, ".nvm", "versions", "node", version, "bin")
		writeTool(t, dir, "node", "#!/bin/sh\n")
		writeTool(t, dir, "claude", "#!/usr/bin/env node\n")
	}
	dirs := standardDirs(home)
	// Isolate the fixture's manager paths from globally installed tools.
	manager := []string{}
	for _, dir := range dirs {
		if strings.HasPrefix(dir, filepath.Join(home, ".nvm")) {
			manager = append(manager, dir)
		}
	}
	resolver := newResolver([]string{"PATH=/absent"}, home, manager, config{})
	for _, name := range []string{"node", "claude"} {
		selection, err := resolver.Resolve(name)
		if err != nil || !strings.Contains(selection.Path, "v22.10.0") {
			t.Fatalf("wrong version: %+v %v", selection, err)
		}
	}
	pinned := filepath.Join(home, ".nvm", "versions", "node", "v9.11.0", "bin")
	resolver = newResolver([]string{"PATH=" + pinned}, home, manager, config{})
	selection, _ := resolver.Resolve("node")
	if selection.Path != filepath.Join(pinned, "node") {
		t.Fatal("standard version overrode PATH")
	}
}

func TestConfigLoadingAndSnapshot(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, "settings.json")
	if err := os.WriteFile(path, []byte(`{"tools":{"node":"~/custom/node"}}`), 0600); err != nil {
		t.Fatal(err)
	}
	settings, err := readConfig([]string{"SESSWITCH_CONFIG=" + path}, home)
	if err != nil || settings.Tools["node"] != "~/custom/node" {
		t.Fatalf("config=%+v err=%v", settings, err)
	}
	if _, err := readConfig([]string{"SESSWITCH_CONFIG=" + path + "-missing"}, home); err == nil {
		t.Fatal("missing explicit config ignored")
	}
	if err := os.WriteFile(path, []byte(`{"tools":{"typo":"/bin/false"}}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := readConfig([]string{"SESSWITCH_CONFIG=" + path}, home); err == nil {
		t.Fatal("unknown configured tool ignored")
	}
	dir := filepath.Join(home, "bin")
	writeTool(t, dir, "node", "#!/bin/sh\n")
	environment := []string{"PATH=" + dir, "OTHER=original"}
	resolver := newResolver(environment, home, nil, config{})
	environment[0] = "PATH=/absent"
	copy := resolver.Environment()
	copy[0] = "OTHER=changed"
	if envValue(resolver.Environment(), "OTHER") != "original" {
		t.Fatal("snapshot mutated")
	}
	ctx := WithResolver(context.Background(), resolver)
	if FromContext(Context(ctx)) != resolver {
		t.Fatal("context replaced snapshot")
	}
}

func TestResumeWorksWithGUIEnvironmentOwnedByExistingWezTerm(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, "tools with spaces")
	agent := writeTool(t, dir, "claude", "#!/usr/bin/env node\n")
	node := writeTool(t, filepath.Join(home, "runtime"), "node", "#!/bin/sh\nprintf '%s\\n' \"$1\" \"$2\" \"$3\" \"$PATH\" \"$CODEX_HOME\" \"${CLAUDE_CONFIG_DIR-unset}\"\n")
	// Simulate an existing GUI that ignores the calling CLI's environment.
	writeTool(t, dir, "wezterm", "#!/bin/sh\nwhile [ \"$1\" != -- ]; do shift; done\nshift\nexec /usr/bin/env -i PATH=/absent CLAUDE_CONFIG_DIR=/stale \"$@\"\n")
	resolver := newResolver([]string{"PATH=/usr/bin:/bin", "SESSWITCH_CLAUDE=" + agent, "SESSWITCH_NODE=" + node, "CODEX_HOME=" + filepath.Join(home, "codex home"), "SESSWITCH_WEZTERM=" + filepath.Join(dir, "wezterm")}, home, nil, config{})
	cmd, err := resolver.Command(context.Background(), "wezterm", "start", "--", "claude", "--resume", "thread-1")
	if err != nil {
		t.Fatal(err)
	}
	data, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	want := []string{agent, "--resume", "thread-1", envValue(resolver.Environment(), "PATH"), filepath.Join(home, "codex home"), "unset"}
	if !containsArgument(cmd.Args, "CODEX_HOME="+filepath.Join(home, "codex home")) || !containsArgument(cmd.Args, "CLAUDE_CONFIG_DIR") {
		t.Fatalf("provider configuration lost: %q", cmd.Args)
	}
	if !reflect.DeepEqual(lines, want) {
		t.Fatalf("resume lost runtime: got %q want %q", lines, want)
	}
}

func containsArgument(args []string, target string) bool {
	for _, arg := range args {
		if arg == target {
			return true
		}
	}
	return false
}

func TestNativeAgentNeedsNoNodeAndShebangFlagsArePreserved(t *testing.T) {
	home := t.TempDir()
	native := writeTool(t, home, "codex", "#!/bin/sh\nexit 0\n")
	resolver := newResolver([]string{"PATH=/absent", "SESSWITCH_CODEX=" + native}, home, nil, config{})
	if err := resolver.Check("codex"); err != nil {
		t.Fatalf("native agent required Node: %v", err)
	}
	script := writeTool(t, home, "claude", "#!/usr/bin/env -S node --no-warnings\n")
	flags, needed, err := nodeShebang(script)
	if err != nil || !needed || !reflect.DeepEqual(flags, []string{"--no-warnings"}) {
		t.Fatalf("flags=%q needed=%v err=%v", flags, needed, err)
	}
}
