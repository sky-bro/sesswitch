package toolenv

import (
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

func standardDirs(home string) []string {
	dirs := []string{
		filepath.Join(home, ".local", "bin"), filepath.Join(home, ".npm-global", "bin"),
		filepath.Join(home, ".npm", "bin"), filepath.Join(home, ".npm-packages", "bin"), filepath.Join(home, ".volta", "bin"),
		"/opt/homebrew/bin", "/usr/local/bin", "/usr/bin", "/bin", "/usr/sbin", "/sbin",
	}
	// Inspect installed versions directly; shims can require shell initialization
	// or a project configuration absent from a GUI process.
	for _, pattern := range []string{
		filepath.Join(home, ".local", "share", "mise", "installs", "node", "*", "bin"),
		filepath.Join(home, ".nvm", "versions", "node", "*", "bin"),
		filepath.Join(home, ".asdf", "installs", "nodejs", "*", "bin"),
		filepath.Join(home, ".local", "share", "fnm", "node-versions", "*", "installation", "bin"),
		filepath.Join(home, "Library", "Application Support", "fnm", "node-versions", "*", "installation", "bin"),
		filepath.Join(home, ".fnm", "node-versions", "*", "installation", "bin"),
	} {
		matches, _ := filepath.Glob(pattern)
		sort.Slice(matches, func(i, j int) bool { return newerVersion(versionDirectory(matches[i]), versionDirectory(matches[j])) })
		dirs = append(dirs, matches...)
	}
	return append(dirs, "/Applications/WezTerm.app/Contents/MacOS", filepath.Join(home, "Applications", "WezTerm.app", "Contents", "MacOS"))
}
func versionDirectory(path string) string {
	directory := filepath.Dir(path)
	if filepath.Base(directory) == "installation" {
		directory = filepath.Dir(directory)
	}
	return filepath.Base(directory)
}
func newerVersion(left, right string) bool {
	parse := func(value string) []int {
		parts := strings.Split(strings.TrimPrefix(value, "v"), ".")
		result := make([]int, len(parts))
		for index, part := range parts {
			number, err := strconv.Atoi(part)
			if err != nil {
				return nil
			}
			result[index] = number
		}
		return result
	}
	a, b := parse(left), parse(right)
	if (a == nil) != (b == nil) {
		return a != nil
	}
	for index := 0; index < len(a) || index < len(b); index++ {
		x, y := 0, 0
		if index < len(a) {
			x = a[index]
		}
		if index < len(b) {
			y = b[index]
		}
		if x != y {
			return x > y
		}
	}
	return left > right
}
