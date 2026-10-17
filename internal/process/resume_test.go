package process

import (
	"context"
	"encoding/binary"
	"errors"
	"testing"
)

func TestExplicitResumeID(t *testing.T) {
	const id = "019a0000-1111-7222-8333-444444444444"
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{"explicit", []string{"/opt/codex", "resume", id}, id},
		{"prompt", []string{"codex", "resume", id, "explain this"}, id},
		{"prompt boundary", []string{"codex", "resume", id, "--", "--last"}, id},
		{"last", []string{"codex", "resume", "--last"}, ""},
		{"last positional is prompt", []string{"codex", "resume", id, "--last"}, ""},
		{"last equals", []string{"codex", "resume", id, "--last=true"}, ""},
		{"picker", []string{"codex", "resume"}, ""},
		{"session name", []string{"codex", "resume", "my thread"}, ""},
		{"fork", []string{"codex", "fork", id}, ""},
		{"exec resume", []string{"codex", "exec", "resume", id}, ""},
		{"prompt resembling command", []string{"codex", "please", "resume", id}, ""},
		{"different executable", []string{"node", "resume", id}, ""},
		{"ambiguous option value", []string{"codex", "-c", "resume", id}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := explicitResumeID(tc.args); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestResumedThreadIdentityVerification(t *testing.T) {
	const id = "019a0000-1111-7222-8333-444444444444"
	base := Info{PID: 99, Start: "start", TTY: "/dev/ttys001", Comm: "codex"}
	for _, tc := range []struct {
		name               string
		live               Info
		same               bool
		readErr, verifyErr error
		want               bool
	}{
		{name: "verified", live: base, same: true, want: true},
		{name: "exited"},
		{name: "PID reused", live: base},
		{name: "TTY changed", live: Info{TTY: "/dev/ttys002"}, same: true},
		{name: "arguments inaccessible", readErr: errors.New("denied")},
		{name: "inspection failed", same: true, live: base, verifyErr: errors.New("denied")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			read := func(pid int) ([]string, error) {
				if pid != base.PID {
					t.Fatal("wrong PID")
				}
				return []string{"codex", "resume", id}, tc.readErr
			}
			verify := func(_ context.Context, pid int, start, provider string) (Info, bool, error) {
				if pid != base.PID || start != base.Start || provider != "codex" {
					t.Fatal("wrong identity")
				}
				return tc.live, tc.same, tc.verifyErr
			}
			if got := resumedThreadMatches(context.Background(), base, "codex", id, read, verify); got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
	for _, provider := range []string{"claude", ""} {
		if resumedThreadMatches(context.Background(), base, provider, id, nil, nil) {
			t.Fatal("wrong provider accepted")
		}
	}
}

func TestParseDarwinArgv(t *testing.T) {
	data := make([]byte, 4)
	binary.NativeEndian.PutUint32(data, 4)
	data = append(data, []byte("/path with spaces/codex\x00\x00\x00codex\x00resume\x00thread-id\x00\x00SECRET=private\x00")...)
	args, err := parseDarwinArgv(data)
	if err != nil || len(args) != 4 || args[0] != "codex" || args[3] != "" {
		t.Fatalf("args=%q, err=%v", args, err)
	}
	for _, data := range [][]byte{nil, {1, 0}, {0, 0, 0, 0}, {255, 255, 255, 255}, {1, 0, 0, 0, 'x'}, {1, 0, 0, 0, 'x', 0, 'y'}} {
		if _, err := parseDarwinArgv(data); err == nil {
			t.Fatalf("malformed arguments accepted: %v", data)
		}
	}
}
