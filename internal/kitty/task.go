package kitty

import (
	"errors"
	"os"
	"os/exec"
	"strconv"
)

func CurrentTab(windows []OSWindow, self WindowID) (TabID, error) {
	for _, osw := range windows {
		for _, tab := range osw.Tabs {
			for _, w := range tab.Windows {
				if w.ID == self {
					return tab.ID, nil
				}
			}
		}
	}
	for _, osw := range windows {
		for _, tab := range osw.Tabs {
			if tab.Focused {
				return tab.ID, nil
			}
		}
	}
	return 0, errors.New("current tab is missing")
}

func EditTask() error {
	windows, err := List()
	if err != nil {
		return err
	}
	self, _ := strconv.Atoi(os.Getenv("KITTY_WINDOW_ID"))
	tab, err := CurrentTab(windows, WindowID(self))
	if err != nil {
		return err
	}
	path, err := TaskPath(tab)
	if err != nil {
		return err
	}
	cmd := exec.Command("nvim", path)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err = cmd.Run(); err != nil {
		return err
	}
	return nil
}
