package process

import (
	"bytes"
	"fmt"
	"os"
)

func processArgv(pid int) ([]string, error) {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
	if err != nil {
		return nil, err
	}
	parts := bytes.Split(bytes.TrimSuffix(data, []byte{0}), []byte{0})
	args := make([]string, len(parts))
	for i, part := range parts {
		args[i] = string(part)
	}
	return args, nil
}
