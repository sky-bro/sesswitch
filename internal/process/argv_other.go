//go:build !darwin && !linux

package process

import "fmt"

func processArgv(int) ([]string, error) {
	return nil, fmt.Errorf("process arguments unavailable on this platform")
}
