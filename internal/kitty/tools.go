package kitty

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func miseShims() string {
	if dir := os.Getenv("MISE_SHIMS_DIR"); dir != "" {
		if strings.HasPrefix(dir, "~/") {
			home, _ := os.UserHomeDir()
			return filepath.Join(home, dir[2:])
		}
		return dir
	}
	data := os.Getenv("MISE_DATA_DIR")
	if data == "" {
		data = os.Getenv("XDG_DATA_HOME")
		if data == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return ""
			}
			data = filepath.Join(home, ".local", "share")
		}
		data = filepath.Join(data, "mise")
	}
	return filepath.Join(data, "shims")
}

func toolPath(name string) (string, error) {
	if strings.ContainsRune(name, filepath.Separator) {
		path, err := exec.LookPath(name)
		if err != nil {
			return "", fmt.Errorf("required tool %s unavailable: %w", name, err)
		}
		return path, nil
	}
	if dir := miseShims(); filepath.IsAbs(dir) {
		if path, err := exec.LookPath(filepath.Join(dir, name)); err == nil {
			return path, nil
		}
	}
	path, err := exec.LookPath(name)
	if err != nil {
		return "", fmt.Errorf("required tool %s missing from mise shims and PATH", name)
	}
	fmt.Fprintf(os.Stderr, "warning: %s unavailable in mise shims; using %s\n", name, path)
	return path, nil
}

func toolCommand(name string, args ...string) *exec.Cmd {
	path, err := toolPath(name)
	if err != nil {
		cmd := exec.Command(name, args...)
		cmd.Err = err
		return cmd
	}
	return exec.Command(path, args...)
}
