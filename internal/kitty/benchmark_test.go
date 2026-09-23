package kitty

import (
	"os"
	"path/filepath"
	"testing"
)

func BenchmarkRows(b *testing.B) {
	root := b.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0700); err != nil {
		b.Fatal(err)
	}
	cwd := filepath.Join(root, "src", "nested")
	if err := os.MkdirAll(cwd, 0700); err != nil {
		b.Fatal(err)
	}
	b.Setenv("KITTY_REPO_STATE", root)
	windows := []OSWindow{{Tabs: []Tab{{ID: 1, Title: "project"}}}}
	for i := range 64 {
		windows[0].Tabs[0].Windows = append(windows[0].Tabs[0].Windows, Window{ID: WindowID(i + 1), Cwd: cwd, Cmdline: []string{"zsh"}})
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		Rows(windows, 0)
	}
}
