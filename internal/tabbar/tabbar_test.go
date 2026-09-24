package tabbar

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"kitty.conf/internal/kitty"
)

func testBuilder(t *testing.T, tasks map[kitty.TabID]string) *Builder {
	t.Helper()
	return &Builder{
		Home: "/home/u",
		ProjectRoot: func(cwd string) string {
			if root, ok := map[string]string{"/home/u/repo/src": "/home/u/repo", "/home/u/repo": "/home/u/repo"}[cwd]; ok {
				return root
			}
			return ""
		},
		ReadTask:   func(id kitty.TabID) string { return tasks[id] },
		RemoveTask: func(kitty.TabID) error { return nil },
	}
}

func render(t *testing.T, b *Builder, tabs ...TabInput) Response {
	t.Helper()
	out, err := b.Render(Request{Version: Version, Revision: 7, Complete: true, Tabs: tabs})
	if err != nil {
		t.Fatal(err)
	}
	if out.Revision != 7 {
		t.Fatalf("revision %d", out.Revision)
	}
	return out
}

func TestLabels(t *testing.T) {
	b := testBuilder(t, map[kitty.TabID]string{2: "Ship it"})
	for _, tt := range []struct {
		name     string
		tab      TabInput
		active   []Segment
		inactive []Segment
	}{
		{"manual", TabInput{ID: 1, ManualTitle: "x", Title: "Named", Cwd: "/home/u/repo"},
			[]Segment{{" ", 0xFFFFFF}, {"Named", 0xFFFFFF}, {" ", 0xFFFFFF}},
			[]Segment{{" ", 0x999999}, {"Named", 0x999999}, {" ", 0x999999}}},
		{"task", TabInput{ID: 2, Title: "zsh", Cwd: "/home/u/repo"},
			[]Segment{{" ", 0xFFFFFF}, {"Ship it", kitty.Color("Ship it")}, {" ", 0xFFFFFF}},
			[]Segment{{" ", 0x999999}, {"Ship it", dim(kitty.Color("Ship it"))}, {" ", 0x999999}}},
		{"project", TabInput{ID: 3, Title: "zsh", Cwd: "/home/u/repo/src"},
			[]Segment{{" ", 0xFFFFFF}, {"[", 0x888888}, {"repo", kitty.Color("repo")}, {"]", 0x888888}, {" ", 0xFFFFFF}},
			[]Segment{{" ", 0x999999}, {"[", 0x777777}, {"repo", kitty.Color("repo")}, {"]", 0x777777}, {" ", 0x999999}}},
		{"no cwd", TabInput{ID: 4, Title: "bad\n\x1btitle"},
			[]Segment{{" ", 0xFFFFFF}, {"bad  title", 0xFFFFFF}, {" ", 0xFFFFFF}},
			[]Segment{{" ", 0x999999}, {"bad  title", 0x999999}, {" ", 0x999999}}},
		{"home shell", TabInput{ID: 5, Title: "zsh", Cwd: "/home/u", ActiveExe: "/bin/zsh", OldestExe: "-zsh"},
			[]Segment{{" ", 0xFFFFFF}, {"zsh ", 0xD2FFD2}, {" ", 0xFFFFFF}, {"~", 0xFFE6B4}, {" ", 0xFFFFFF}},
			[]Segment{{" ", 0x999999}, {"zsh ", 0xB4DCB4}, {" ", 0xC8C8C8}, {"~", 0xDCC8A0}, {" ", 0x999999}}},
		{"oldest exe wins", TabInput{ID: 6, Title: "nvim main.go", Cwd: "/tmp/", ActiveExe: "zsh", OldestExe: "/usr/bin/nvim"},
			[]Segment{{" ", 0xFFFFFF}, {"nvim ", 0xD2FFD2}, {" ", 0xFFFFFF}, {"tmp", 0xFFE6B4}, {" ", 0xFFFFFF}}, nil},
		{"title word wins", TabInput{ID: 7, Title: "ssh host", Cwd: "/", ActiveExe: "-zsh", OldestExe: "bash"},
			[]Segment{{" ", 0xFFFFFF}, {"ssh ", 0xD2FFD2}, {" ", 0xFFFFFF}, {"/", 0xFFE6B4}, {" ", 0xFFFFFF}}, nil},
		{"title fallback", TabInput{ID: 8, Title: "", Cwd: "/opt/x", ActiveExe: "", OldestExe: ""},
			[]Segment{{" ", 0xFFFFFF}, {" ", 0xD2FFD2}, {" ", 0xFFFFFF}, {"x", 0xFFE6B4}, {" ", 0xFFFFFF}}, nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			styles := render(t, b, tt.tab).Tabs[tt.tab.ID]
			if !reflect.DeepEqual(styles.Active.Segments, tt.active) || styles.Active.Background != 0x1A3A6A {
				t.Errorf("active: %+v", styles.Active)
			}
			if tt.inactive != nil && (!reflect.DeepEqual(styles.Inactive.Segments, tt.inactive) || styles.Inactive.Background != 0x2A2A2A) {
				t.Errorf("inactive: %+v", styles.Inactive)
			}
		})
	}
}

func TestStickyProjectAndPrune(t *testing.T) {
	var removed []kitty.TabID
	b := testBuilder(t, nil)
	b.RemoveTask = func(id kitty.TabID) error { removed = append(removed, id); return nil }
	render(t, b, TabInput{ID: 1, Cwd: "/home/u/repo"}, TabInput{ID: 2, Cwd: "/home/u"})
	out := render(t, b, TabInput{ID: 1, Title: "zsh", Cwd: "/private/tmp"})
	if got := out.Tabs[1].Active.Segments[2].Text; got != "repo" {
		t.Fatalf("sticky project lost: %q", got)
	}
	if !reflect.DeepEqual(removed, []kitty.TabID{2}) {
		t.Fatalf("pruned %v", removed)
	}
	_, err := b.Render(Request{Version: Version, Complete: false})
	if err != nil || len(removed) != 1 || b.lastProject[1] == "" {
		t.Fatal("partial snapshot pruned state")
	}
	if _, err := b.Render(Request{Version: 99}); err == nil {
		t.Fatal("version accepted")
	}
}

func TestDefaultBuilderRemovesTaskFiles(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("KITTY_REPO_STATE", dir)
	path := filepath.Join(dir, "task-3")
	if err := os.WriteFile(path, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	b := NewBuilder()
	render(t, b, TabInput{ID: 3})
	if got := render(t, b, TabInput{ID: 3}).Tabs[3].Active.Segments[1].Text; got != "x" {
		t.Fatalf("task ignored: %q", got)
	}
	render(t, b)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("task file kept")
	}
	if err := b.RemoveTask(3); err != nil {
		t.Fatal(err)
	}
}
