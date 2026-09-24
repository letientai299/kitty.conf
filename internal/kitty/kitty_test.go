package kitty

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestProjectRoot(t *testing.T) {
	for _, worktree := range []bool{false, true} {
		root := t.TempDir()
		var err error
		if worktree {
			err = os.WriteFile(filepath.Join(root, ".git"), []byte("gitdir: ../main/.git/worktrees/test\n"), 0600)
		} else {
			err = os.Mkdir(filepath.Join(root, ".git"), 0700)
		}
		if err != nil {
			t.Fatal(err)
		}
		child := filepath.Join(root, "src", "pkg")
		if err = os.MkdirAll(child, 0700); err != nil {
			t.Fatal(err)
		}
		if got := ProjectRoot(child); got != root {
			t.Fatalf("got %q, want %q", got, root)
		}
	}
	if ProjectRoot("") != "" {
		t.Fatal("empty cwd resolved")
	}
}

func TestTaskIsolation(t *testing.T) {
	first, second := t.TempDir(), t.TempDir()
	t.Setenv("KITTY_REPO_STATE", first)
	path, err := TaskPath(1)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, []byte("  task one\nignored"), 0600); err != nil {
		t.Fatal(err)
	}
	if ReadTask(1) != "task one" {
		t.Fatal("first line lost")
	}
	t.Setenv("KITTY_REPO_STATE", second)
	if ReadTask(1) != "" {
		t.Fatal("task leaked across instances")
	}
	t.Setenv("KITTY_REPO_STATE", "")
	if _, err = TaskPath(1); err == nil {
		t.Fatal("missing state accepted")
	}
}

func TestCurrentTab(t *testing.T) {
	windows := []OSWindow{{Tabs: []Tab{{ID: 1, Focused: true, Windows: []Window{{ID: 10}}}, {ID: 2, Windows: []Window{{ID: 20}}}}}}
	for _, tt := range []struct {
		self WindowID
		want TabID
	}{{20, 2}, {99, 1}} {
		got, err := CurrentTab(windows, tt.self)
		if err != nil || got != tt.want {
			t.Fatalf("got %d, %v", got, err)
		}
	}
	if _, err := CurrentTab(nil, 1); err == nil {
		t.Fatal("missing tab accepted")
	}
}

func TestRows(t *testing.T) {
	t.Setenv("KITTY_REPO_STATE", t.TempDir())
	windows := []OSWindow{{Tabs: []Tab{{ID: 1, Title: "tab\n\x1b[31m", Windows: []Window{{ID: 10, Cmdline: []string{"zsh"}}, {ID: 20, Title: "editor\tname", Active: true, Foreground: []Process{{Cmdline: []string{"nvim"}}}}, {ID: 99, Cmdline: []string{"kt", "switch"}}}}}}}
	rows := Rows(windows, 99)
	if len(rows) != 4 {
		t.Fatalf("rows: %v", rows)
	}
	for _, row := range rows[:2] {
		if !strings.HasPrefix(row, "20\t") {
			t.Fatalf("inactive pane targeted: %q", row)
		}
	}
	for _, row := range rows {
		if len(strings.Split(row, "\t")) != 3 || strings.Contains(row, "\n") {
			t.Fatalf("unsafe row: %q", row)
		}
		if strings.HasPrefix(row, "99\t") {
			t.Fatal("overlay included")
		}
	}
	if !strings.Contains(rows[3], "editor name") {
		t.Fatal("editor title lost")
	}
}

func TestLaunchEnv(t *testing.T) {
	env := launchEnv([]string{"HOME=/home/a", "KITTY_LISTEN_ON=old", "KITTY_WINDOW_ID=2", "KITTY_PID=3", "KITTY_CONFIG_DIRECTORY=old", "KITTY_REPO_STATE=old"}, "/new config", "/new state", "/kitten")
	for _, want := range []string{"HOME=/home/a", "KITTY_CONFIG_DIRECTORY=/new config/conf", "KITTY_REPO_ROOT=/new config", "KITTY_REPO_STATE=/new state", "KITTY_REPO_KITTEN=/kitten"} {
		if !slices.Contains(env, want) {
			t.Fatalf("missing %s", want)
		}
	}
	for _, value := range env {
		if strings.Contains(value, "=old") || strings.HasPrefix(value, "KITTY_LISTEN_ON=") {
			t.Fatalf("inherited instance: %s", value)
		}
	}
}

func TestColor(t *testing.T) {
	for _, tt := range []struct {
		name string
		want uint32
	}{{"", 0xff6e6e}, {"kitty.conf", 0xff79c6}, {"任务", 0xf1fa8c}} {
		if got := Color(tt.name); got != tt.want {
			t.Errorf("%q: got %#x, want %#x", tt.name, got, tt.want)
		}
	}
}

func fakeCommand(t *testing.T, dir, name, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\nset -eu\n"+body), 0700); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestRemoteTarget(t *testing.T) {
	dir := t.TempDir()
	binary := fakeCommand(t, dir, "kitten", "printf '%s\\n' \"$@\"\n")
	t.Setenv("KITTY_REPO_KITTEN", binary)
	t.Setenv("KITTY_LISTEN_ON", "unix:/private/socket")
	out, err := Remote("ls")
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "@\n--to\nunix:/private/socket\nls\n" {
		t.Fatalf("arguments: %q", out)
	}
	t.Setenv("KITTY_LISTEN_ON", "")
	if _, err = Remote("ls"); err == nil {
		t.Fatal("missing socket accepted")
	}
}

func TestSwitchFocus(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("KITTY_LISTEN_ON", "unix:/isolated")
	t.Setenv("KITTY_REPO_STATE", dir)
	t.Setenv("KITTY_WINDOW_ID", "99")
	t.Setenv("TEST_RECORD", filepath.Join(dir, "record"))
	binary := fakeCommand(t, dir, "kitten", `if [ "$4" = ls ]; then
 printf '%s' '[{"tabs":[{"id":1,"title":"tab","windows":[{"id":10,"cmdline":["zsh"]},{"id":99}]}]}]'
else
 printf '%s\n' "$@" > "$TEST_RECORD"
fi
`)
	t.Setenv("KITTY_REPO_KITTEN", binary)
	fakeCommand(t, dir, "fzf", "cat >/dev/null\nprintf '10\\tpane\\tselected\\n'\n")
	if err := Switch(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(os.Getenv("TEST_RECORD"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "focus-window\n--match\nid:10\n") {
		t.Fatalf("focus: %s", data)
	}
	if err = os.Remove(os.Getenv("TEST_RECORD")); err != nil {
		t.Fatal(err)
	}
	fakeCommand(t, dir, "fzf", "cat >/dev/null\nexit 130\n")
	if err = Switch(); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(os.Getenv("TEST_RECORD")); !os.IsNotExist(err) {
		t.Fatal("cancel focused a window")
	}
}

func TestBroadcastTarget(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("KITTY_REPO_KITTEN", fakeCommand(t, dir, "kitten", `printf '%s ' "$@" >> "$TEST_RECORD"
printf '\n' >> "$TEST_RECORD"
`))
	t.Setenv("TEST_RECORD", filepath.Join(dir, "record"))
	t.Setenv("KITTY_LISTEN_ON", "unix:/private")
	t.Setenv("KITTY_WINDOW_ID", "7")
	if err := Broadcast(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(os.Getenv("TEST_RECORD"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(data), "launch --type=window --keep-focus") != 4 {
		t.Fatalf("launches: %s", data)
	}
	if !strings.Contains(string(data), "goto-layout --match window_id:7 grid") || !strings.Contains(string(data), "focus-window --match id:7") {
		t.Fatalf("target: %s", data)
	}
}
