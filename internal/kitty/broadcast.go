package kitty

import (
	"errors"
	"os"
	"strconv"
	"strings"
)

type BroadcastOptions struct {
	New  int
	Pick bool
}

func Broadcast(opts BroadcastOptions) error {
	if opts.New < 0 {
		return errors.New("pane count must not be negative")
	}
	windows, err := List()
	if err != nil {
		return err
	}
	self, _ := strconv.Atoi(os.Getenv("KITTY_WINDOW_ID"))
	var targets []WindowID
	if opts.Pick {
		targets, err = pickPanes(windows, WindowID(self))
		if err != nil {
			return err
		}
		if len(targets) == 0 {
			return nil
		}
	} else {
		tab, err := CurrentTab(windows, WindowID(self))
		if err != nil {
			return err
		}
		for _, w := range tab.Windows {
			targets = append(targets, w.ID)
		}
	}
	for range opts.New {
		out, err := Remote("launch", "--type=window", "--keep-focus")
		if err != nil {
			return err
		}
		id, err := strconv.Atoi(strings.TrimSpace(string(out)))
		if err != nil {
			return errors.New("launch returned no window ID")
		}
		targets = append(targets, WindowID(id))
	}
	if opts.New > 0 && self > 0 {
		if _, err := Remote("goto-layout", "--match", "window_id:"+strconv.Itoa(self), "grid"); err != nil {
			return err
		}
		if _, err := Remote("focus-window", "--match", Match(WindowID(self))); err != nil {
			return err
		}
	}
	if len(targets) < 2 {
		return errors.New("need two panes; use --new or --pick")
	}
	matches := make([]string, len(targets))
	for i, id := range targets {
		matches[i] = Match(id)
	}
	cmd := toolCommand(kittenBinary(), "broadcast", "--match", strings.Join(matches, " or "))
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd.Run()
}
