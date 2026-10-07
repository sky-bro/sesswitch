package process

import (
	"fmt"
	"strings"
	"testing"
)

func TestAgentFromTTYOutput(t *testing.T) {
	row := func(pid, parent int, command string) string {
		return fmt.Sprintf("%d %d ttys004 Mon Oct 5 10:06:19 2026 %s\n", pid, parent, command)
	}
	for _, test := range []struct {
		name, data string
		wantPID    int
	}{
		{"helper before parent", row(10, 20, "/app/codex") + row(20, 30, "/app/node") + row(30, 40, "/bin/codex") + row(40, 1, "-zsh"), 30},
		{"multiple helpers", row(10, 30, "/app/codex") + row(11, 30, "/app/codex") + row(30, 1, "/bin/codex"), 30},
		{"independent agents", row(10, 1, "/bin/codex") + row(30, 1, "/bin/codex"), 0},
		{"different provider", row(10, 1, "/bin/claude") + row(30, 1, "/bin/codex"), 30},
		{"different tty", strings.ReplaceAll(row(30, 1, "/bin/codex"), "ttys004", "ttys005"), 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			info, found, err := agentFromTTYOutput(test.data, "/dev/ttys004", "codex")
			if err != nil || found != (test.wantPID != 0) || info.PID != test.wantPID {
				t.Fatalf("got %+v, found=%v, err=%v; want PID %d", info, found, err, test.wantPID)
			}
		})
	}
}

func TestParsePS(t *testing.T) {
	info, err := Parse(95099, "95098 ttys015  Fri Sep 18 23:00:42 2026     /opt/bin/codex\n")
	if err != nil {
		t.Fatal(err)
	}
	if info.PPID != 95098 || info.TTY != "/dev/ttys015" || info.Start != "Fri Sep 18 23:00:42 2026" || !IsCodex(info) {
		t.Fatalf("unexpected info: %+v", info)
	}
}

func TestNormalizeTTY(t *testing.T) {
	if NormalizeTTY("??") != "" || NormalizeTTY("/dev/pts/2") != "/dev/pts/2" {
		t.Fatal("TTY normalization failed")
	}
}

func TestIsAgentByProvider(t *testing.T) {
	if !IsAgent(Info{Comm: "/opt/bin/codex"}, "codex") {
		t.Fatal("codex not recognized")
	}
	if !IsAgent(Info{Comm: "/opt/bin/claude"}, "claude") {
		t.Fatal("claude not recognized")
	}
	if IsAgent(Info{Comm: "/opt/bin/codex"}, "claude") {
		t.Fatal("provider mismatch accepted")
	}
}
