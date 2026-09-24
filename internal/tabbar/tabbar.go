// Package tabbar computes tab labels and colors for conf/tab_bar.py.
package tabbar

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"kitty.conf/internal/kitty"
)

const Version = 1

type TabInput struct {
	ID          kitty.TabID `json:"id"`
	OSWindowID  int         `json:"os_window_id"`
	ManualTitle string      `json:"manual_title"`
	Title       string      `json:"title"`
	Cwd         string      `json:"cwd"`
	ActiveExe   string      `json:"active_exe"`
	OldestExe   string      `json:"oldest_exe"`
}

type Request struct {
	Version  int        `json:"version"`
	Revision uint64     `json:"revision"`
	Complete bool       `json:"complete"`
	Tabs     []TabInput `json:"tabs"`
}

type Segment struct {
	Text  string `json:"text"`
	Color uint32 `json:"color"`
}

type Style struct {
	Background uint32    `json:"background"`
	Segments   []Segment `json:"segments"`
}

type TabStyles struct {
	Active   Style `json:"active"`
	Inactive Style `json:"inactive"`
}

type Response struct {
	Version  int                       `json:"version"`
	Revision uint64                    `json:"revision"`
	Tabs     map[kitty.TabID]TabStyles `json:"tabs,omitempty"`
	Error    string                    `json:"error,omitempty"`
}

// Builder keeps per-tab history between snapshots.
type Builder struct {
	Home        string
	ProjectRoot func(cwd string) string
	ReadTask    func(id kitty.TabID) string
	RemoveTask  func(id kitty.TabID) error
	lastProject map[kitty.TabID]string
	seen        map[kitty.TabID]bool
}

func NewBuilder() *Builder {
	home, _ := os.UserHomeDir()
	return &Builder{
		Home:        home,
		ProjectRoot: kitty.ProjectRoot,
		ReadTask:    kitty.ReadTask,
		RemoveTask: func(id kitty.TabID) error {
			path, err := kitty.TaskPath(id)
			if err != nil {
				return err
			}
			err = os.Remove(path)
			if os.IsNotExist(err) {
				return nil
			}
			return err
		},
	}
}

func (b *Builder) Render(req Request) (Response, error) {
	if req.Version != Version {
		return Response{}, fmt.Errorf("unsupported protocol version %d", req.Version)
	}
	if b.lastProject == nil {
		b.lastProject = map[kitty.TabID]string{}
		b.seen = map[kitty.TabID]bool{}
	}
	live := make(map[kitty.TabID]bool, len(req.Tabs))
	out := Response{Version: Version, Revision: req.Revision, Tabs: make(map[kitty.TabID]TabStyles, len(req.Tabs))}
	for i := range req.Tabs {
		tab := &req.Tabs[i]
		live[tab.ID] = true
		out.Tabs[tab.ID] = TabStyles{Active: b.style(tab, true), Inactive: b.style(tab, false)}
	}
	if req.Complete {
		b.forget(live)
	}
	for id := range live {
		b.seen[id] = true
	}
	return out, nil
}

// Closed tabs release history and task files.
func (b *Builder) forget(live map[kitty.TabID]bool) {
	for id := range b.seen {
		if live[id] {
			continue
		}
		delete(b.seen, id)
		delete(b.lastProject, id)
		if err := b.RemoveTask(id); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	}
}

func (b *Builder) style(tab *TabInput, active bool) Style {
	foreground := pick(active, 0xFFFFFF, 0x999999)
	label := b.label(tab, active, foreground)
	segments := make([]Segment, 0, len(label)+2)
	segments = append(segments, Segment{" ", foreground})
	segments = append(segments, label...)
	segments = append(segments, Segment{" ", foreground})
	for i := range segments {
		segments[i].Text = kitty.Clean(segments[i].Text)
	}
	return Style{Background: pick(active, 0x1A3A6A, 0x2A2A2A), Segments: segments}
}

func (b *Builder) label(tab *TabInput, active bool, foreground uint32) []Segment {
	if tab.ManualTitle != "" {
		return []Segment{{tab.Title, foreground}}
	}
	if task := b.ReadTask(tab.ID); task != "" {
		color := kitty.Color(task)
		if !active {
			color = dim(color)
		}
		return []Segment{{task, color}}
	}
	project := ""
	if tab.Cwd != "" {
		if root := b.ProjectRoot(tab.Cwd); root != "" {
			project = filepath.Base(root)
		}
	}
	if project != "" {
		b.lastProject[tab.ID] = project
	} else {
		project = b.lastProject[tab.ID]
	}
	if project != "" {
		brackets := pick(active, 0x888888, 0x777777)
		return []Segment{{"[", brackets}, {project, kitty.Color(project)}, {"]", brackets}}
	}
	if tab.Cwd == "" {
		return []Segment{{tab.Title, foreground}}
	}
	return []Segment{
		{processName(tab) + " ", pick(active, 0xD2FFD2, 0xB4DCB4)},
		{" ", pick(active, 0xFFFFFF, 0xC8C8C8)},
		{b.folder(tab.Cwd), pick(active, 0xFFE6B4, 0xDCC8A0)},
	}
}

func (b *Builder) folder(cwd string) string {
	if cwd == b.Home {
		return "~"
	}
	trimmed := strings.TrimRight(cwd, "/")
	if trimmed == "" {
		return "/"
	}
	return filepath.Base(trimmed)
}

var boringShells = map[string]bool{"sh": true, "bash": true, "zsh": true, "fish": true, "dash": true, "ksh": true, "tcsh": true, "csh": true}

func exeName(path string) string {
	if path == "" {
		return ""
	}
	return strings.TrimLeft(filepath.Base(path), "-")
}

func processName(tab *TabInput) string {
	process := tab.ActiveExe
	if process == "" {
		process = tab.OldestExe
	}
	if process == "" {
		process = tab.Title
	}
	if boringShells[exeName(process)] {
		words := strings.Fields(tab.Title)
		switch {
		case tab.OldestExe != "" && !boringShells[exeName(tab.OldestExe)]:
			process = tab.OldestExe
		case len(words) > 0 && !boringShells[words[0]]:
			process = words[0]
		}
	}
	return exeName(process)
}

// Inactive task colors keep 45% of each channel.
func dim(color uint32) uint32 {
	var out uint32
	for _, shift := range []uint32{16, 8, 0} {
		out |= uint32(float64((color>>shift)&255)*0.45) << shift
	}
	return out
}

func pick(active bool, on, off uint32) uint32 {
	if active {
		return on
	}
	return off
}
