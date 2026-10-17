package process

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"path/filepath"
	"strings"
)

// ResumedThreadMatches supplies additional recovery evidence, never a complete
// foreground-thread assertion: argv cannot reflect later interactive switches.
// Callers must also check current UI evidence and competing session leases.
// Arguments are read on demand and are never stored or logged.
func ResumedThreadMatches(ctx context.Context, info Info, provider, id string) bool {
	return resumedThreadMatches(ctx, info, provider, id, processArgv, SameIdentityFor)
}

func resumedThreadMatches(ctx context.Context, info Info, provider, id string,
	read func(int) ([]string, error), verify func(context.Context, int, string, string) (Info, bool, error),
) bool {
	if provider != "codex" || info.PID <= 0 || info.Start == "" || info.TTY == "" || !IsCodex(info) || !threadUUID(id) {
		return false
	}
	args, err := read(info.PID)
	if err != nil || explicitResumeID(args) != id {
		return false
	}
	// Reject exit, PID reuse, or a TTY change during argument inspection.
	live, same, err := verify(ctx, info.PID, info.Start, provider)
	return err == nil && same && live.TTY == info.TTY
}

func explicitResumeID(args []string) string {
	// Deliberately recognize only an unambiguous interactive invocation. Do
	// not scan arbitrary prompts/config values for the word "resume".
	if len(args) < 3 || filepath.Base(args[0]) != "codex" || args[1] != "resume" || !threadUUID(args[2]) {
		return ""
	}
	for _, arg := range args[3:] {
		if arg == "--" {
			break // remaining arguments are a prompt, not options
		}
		if arg == "--last" || strings.HasPrefix(arg, "--last=") {
			return ""
		}
	}
	return args[2]
}

func threadUUID(id string) bool {
	if len(id) != 36 {
		return false
	}
	for i, c := range id {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if c != '-' {
				return false
			}
		} else if !strings.ContainsRune("0123456789abcdefABCDEF", c) {
			return false
		}
	}
	return true
}

// Darwin's KERN_PROCARGS2 contains argc, an executable path, NUL padding,
// exactly argc NUL-terminated arguments, then environment entries. Never parse
// the environment as argv, and preserve empty argument boundaries.
func parseDarwinArgv(data []byte) ([]string, error) {
	if len(data) < 4 {
		return nil, fmt.Errorf("missing argument count")
	}
	argc := int(binary.NativeEndian.Uint32(data[:4]))
	data = data[4:]
	if argc <= 0 || argc > len(data) {
		return nil, fmt.Errorf("invalid argument count")
	}
	end := bytes.IndexByte(data, 0)
	if end < 0 {
		return nil, fmt.Errorf("missing executable path")
	}
	data = bytes.TrimLeft(data[end+1:], "\x00")
	args := make([]string, 0, argc)
	for range argc {
		end = bytes.IndexByte(data, 0)
		if end < 0 {
			return nil, fmt.Errorf("incomplete arguments")
		}
		args = append(args, string(data[:end]))
		data = data[end+1:]
	}
	return args, nil
}
