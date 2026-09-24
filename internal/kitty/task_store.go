package kitty

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func TaskPath(id TabID) (string, error) {
	state := os.Getenv("KITTY_REPO_STATE")
	if state == "" || id <= 0 {
		return "", errors.New("task state or tab ID is invalid")
	}
	return filepath.Join(state, fmt.Sprintf("task-%d", id)), nil
}

func ReadTask(id TabID) string {
	path, err := TaskPath(id)
	if err != nil {
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	line, _, _ := strings.Cut(strings.TrimSpace(string(data)), "\n")
	return strings.TrimSpace(line)
}
