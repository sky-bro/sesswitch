package process

import "testing"

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
