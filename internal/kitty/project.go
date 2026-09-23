package kitty

import (
	"os"
	"path/filepath"
	"strings"
)

func ProjectRoot(cwd string) string {
	if cwd == "" {
		return ""
	}
	path, err := filepath.Abs(cwd)
	if err != nil {
		return ""
	}
	for {
		git := filepath.Join(path, ".git")
		if info, err := os.Stat(git); err == nil {
			if info.IsDir() {
				return path
			}
			if data, err := os.ReadFile(git); err == nil && strings.HasPrefix(string(data), "gitdir: ") {
				return path
			}
		}
		parent := filepath.Dir(path)
		if parent == path {
			return ""
		}
		path = parent
	}
}
