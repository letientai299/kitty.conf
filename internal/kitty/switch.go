package kitty

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"unicode"
)

func paint(color uint32, value string) string {
	return fmt.Sprintf("\x1b[38;2;%d;%d;%dm%s\x1b[0m", color>>16, (color>>8)&255, color&255, Clean(value))
}

func processName(w *Window) string {
	for _, p := range slices.Backward(w.Foreground) {
		cmd := p.Cmdline
		if len(cmd) == 0 {
			continue
		}
		name := strings.TrimLeft(filepath.Base(cmd[0]), "-")
		if name == "fzf" || name == "kt" {
			continue
		}
		return name
	}
	if len(w.Cmdline) > 0 {
		return strings.TrimLeft(filepath.Base(w.Cmdline[0]), "-")
	}
	if w.Title != "" {
		return w.Title
	}
	return "process"
}

func target(panes []Window) WindowID {
	for _, w := range panes {
		if w.Active {
			return w.ID
		}
	}
	return panes[0].ID
}

// The switcher overlay must not list itself.
func isOverlay(w *Window) bool {
	return len(w.Cmdline) > 1 && filepath.Base(w.Cmdline[0]) == "kt"
}

func Rows(windows []OSWindow, self WindowID) []string {
	var rows []string
	roots := make(map[string]string)
	projectRoot := func(cwd string) string {
		if root, ok := roots[cwd]; ok {
			return root
		}
		root := ProjectRoot(cwd)
		roots[cwd] = root
		return root
	}
	for _, osw := range windows {
		var body []string
		var projects []string
		seen := map[string]bool{}
		var first WindowID
		for _, tab := range osw.Tabs {
			var panes []Window
			project := ""
			for _, w := range tab.Windows {
				if w.ID == self && isOverlay(&w) {
					continue
				}
				panes = append(panes, w)
				root := projectRoot(w.Cwd)
				if root != "" && (project == "" || w.Active || w.Focused) {
					project = filepath.Base(root)
				}
			}
			if len(panes) == 0 {
				continue
			}
			title := ReadTask(tab.ID)
			color := uint32(0x6ea8fe)
			switch {
			case title != "":
				color = Color(title)
			case project != "":
				title = "[" + project + "]"
				color = Color(project)
			case tab.Title != "":
				title = tab.Title
			default:
				title = "Tab"
			}
			if rest, found := strings.CutPrefix(title, "["); found {
				p, _, ok := strings.Cut(rest, "]")
				if ok && p != "" && !seen[p] {
					projects = append(projects, p)
					seen[p] = true
				}
			}
			focus := target(panes)
			if first == 0 || tab.Active {
				first = focus
			}
			body = append(body, fmt.Sprintf("%d\ttab\t  %s", focus, paint(color, title)))
			for i, w := range panes {
				repo := paint(0x666666, "-")
				if root := projectRoot(w.Cwd); root != "" {
					repo = paint(0x8be9fd, filepath.Base(root))
					if rel, err := filepath.Rel(root, w.Cwd); err == nil && rel != "." {
						repo += " " + paint(0x50fa7b, rel)
					}
				}
				name := strings.TrimSpace(Clean(processName(&w)))
				label := paint(0xbd93f9, name)
				if title := strings.TrimSpace(Clean(w.Title)); title != "" && title != name {
					label += " — " + paint(0x999999, title)
				}
				body = append(body, fmt.Sprintf("%d\tpane\t    %s %s %s", w.ID, paint(0xf1fa8c, fmt.Sprintf("%2d", i+1)), repo, label))
			}
		}
		if len(body) == 0 {
			continue
		}
		label := strings.Join(projects, ", ")
		if label == "" {
			label = "Window"
		}
		rows = append(rows, fmt.Sprintf("%d\tos\t%s", first, paint(0xff79c6, label)))
		rows = append(rows, body...)
	}
	return rows
}

func Switch() error {
	windows, err := List()
	if err != nil {
		return err
	}
	self, _ := strconv.Atoi(os.Getenv("KITTY_WINDOW_ID"))
	rows := Rows(windows, WindowID(self))
	if len(rows) == 0 {
		return errors.New("no panes found")
	}
	ids, err := selectRows(rows, "Go to > ", false)
	if err != nil || len(ids) == 0 {
		return err
	}
	_, err = Remote("focus-window", "--match", Match(ids[0]))
	return err
}

func pickPanes(windows []OSWindow, self WindowID) ([]WindowID, error) {
	var rows []string
	for _, row := range Rows(windows, self) {
		if strings.Contains(row, "\tpane\t") {
			rows = append(rows, row)
		}
	}
	if len(rows) == 0 {
		return nil, errors.New("no panes found")
	}
	return selectRows(rows, "Broadcast to > ", true)
}

// Cancelling fzf returns no IDs and no error.
func selectRows(rows []string, prompt string, multi bool) ([]WindowID, error) {
	binary, err := os.Executable()
	if err != nil {
		return nil, err
	}
	preview := "'" + strings.ReplaceAll(binary, "'", "'\"'\"'") + "' preview {1}"
	args := []string{"--ansi", "--delimiter", "\t", "--with-nth", "3", "--header", "Select Kitty pane", "--layout=reverse", "--border", "--prompt", prompt, "--preview", preview, "--preview-window", "down,65%,border-top"}
	if multi {
		args = append(args, "--multi", "--header", "Tab marks panes; Enter confirms")
	}
	cmd := exec.Command("fzf", args...)
	cmd.Env = append(os.Environ(), "SHELL=/bin/sh", "FZF_DEFAULT_OPTS=", "FZF_DEFAULT_OPTS_FILE=")
	cmd.Stdin = strings.NewReader(strings.Join(rows, "\n"))
	cmd.Stderr = os.Stderr
	output, err := cmd.Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) && (exit.ExitCode() == 1 || exit.ExitCode() == 130) {
			return nil, nil
		}
		return nil, err
	}
	var ids []WindowID
	for line := range strings.SplitSeq(strings.TrimSpace(string(output)), "\n") {
		selected, _, _ := strings.Cut(line, "\t")
		id, err := strconv.Atoi(selected)
		if err != nil || id <= 0 {
			return nil, errors.New("selected window ID is invalid")
		}
		ids = append(ids, WindowID(id))
	}
	return ids, nil
}

func Clean(text string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, text)
}
