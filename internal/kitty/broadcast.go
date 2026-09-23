package kitty

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
)

func Broadcast() error {
	binary := os.Getenv("KITTY_REPO_KITTEN")
	self, err := strconv.Atoi(os.Getenv("KITTY_WINDOW_ID"))
	if binary == "" || err != nil || self <= 0 {
		return fmt.Errorf("broadcast instance or window is missing")
	}
	for range 4 {
		if _, err := Remote("launch", "--type=window", "--keep-focus"); err != nil {
			return err
		}
	}
	if _, err := Remote("goto-layout", "--match", "window_id:"+strconv.Itoa(self), "grid"); err != nil {
		return err
	}
	if _, err := Remote("focus-window", "--match", Match(WindowID(self))); err != nil {
		return err
	}
	cmd := exec.Command(binary, "broadcast")
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd.Run()
}
