package kitty

import (
	"errors"
	"os"
	"strconv"
)

func CurrentTab(windows []OSWindow, self WindowID) (*Tab, error) {
	for _, osw := range windows {
		for i := range osw.Tabs {
			for _, w := range osw.Tabs[i].Windows {
				if w.ID == self {
					return &osw.Tabs[i], nil
				}
			}
		}
	}
	for _, osw := range windows {
		for i := range osw.Tabs {
			if osw.Tabs[i].Focused {
				return &osw.Tabs[i], nil
			}
		}
	}
	return nil, errors.New("current tab is missing")
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
	path, err := TaskPath(tab.ID)
	if err != nil {
		return err
	}
	cmd := toolCommand("nvim", path)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd.Run()
}
