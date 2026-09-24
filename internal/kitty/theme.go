package kitty

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
)

func Theme(name string) error {
	root := os.Getenv("KITTY_REPO_ROOT")
	config := os.Getenv("KITTY_CONFIG_DIRECTORY")
	if root == "" || config == "" || filepath.Clean(config) != filepath.Join(root, "conf") {
		return errors.New("theme requires a kt instance")
	}
	local := filepath.Join(config, "local")
	if err := os.MkdirAll(local, 0o700); err != nil {
		return err
	}
	args := []string{"themes", "--config-file-name", "theme.conf", "--reload-in", "parent"}
	if name != "" {
		args = append(args, "--", name)
	}
	cmd := exec.Command(kittenBinary(), args...)
	cmd.Env = append(os.Environ(), "KITTY_CONFIG_DIRECTORY="+local)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd.Run()
}
