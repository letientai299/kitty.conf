package kitty

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"strconv"
)

type WindowID int
type TabID int

type Process struct {
	Cmdline []string `json:"cmdline"`
}
type Window struct {
	ID         WindowID  `json:"id"`
	Cwd        string    `json:"cwd"`
	Title      string    `json:"title"`
	Cmdline    []string  `json:"cmdline"`
	Foreground []Process `json:"foreground_processes"`
	Active     bool      `json:"is_active"`
	Focused    bool      `json:"is_focused"`
}
type Tab struct {
	ID      TabID    `json:"id"`
	Title   string   `json:"title"`
	Active  bool     `json:"is_active"`
	Focused bool     `json:"is_focused"`
	Windows []Window `json:"windows"`
}
type OSWindow struct {
	Tabs []Tab `json:"tabs"`
}

func Remote(args ...string) ([]byte, error) {
	socket := os.Getenv("KITTY_LISTEN_ON")
	if socket == "" {
		return nil, errors.New("KITTY_LISTEN_ON is missing")
	}
	cmd := exec.Command(kittenBinary(), append([]string{"@", "--to", socket}, args...)...)
	cmd.Stderr = os.Stderr
	return cmd.Output()
}

func kittenBinary() string {
	if binary := os.Getenv("KITTY_REPO_KITTEN"); binary != "" {
		return binary
	}
	return "kitten"
}

func List() ([]OSWindow, error) {
	data, err := Remote("ls")
	if err != nil {
		return nil, err
	}
	var windows []OSWindow
	err = json.Unmarshal(data, &windows)
	return windows, err
}

func Match(id WindowID) string { return "id:" + strconv.Itoa(int(id)) }
