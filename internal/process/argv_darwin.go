package process

import "golang.org/x/sys/unix"

func processArgv(pid int) ([]string, error) {
	data, err := unix.SysctlRaw("kern.procargs2", pid)
	if err != nil {
		return nil, err
	}
	return parseDarwinArgv(data)
}
