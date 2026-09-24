package kitty

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestThemePicker(t *testing.T) {
	root := t.TempDir()
	config := filepath.Join(root, "conf")
	record := filepath.Join(root, "record")
	t.Setenv("KITTY_REPO_ROOT", root)
	t.Setenv("KITTY_CONFIG_DIRECTORY", config)
	t.Setenv("TEST_RECORD", record)
	t.Setenv("KITTY_REPO_KITTEN", fakeCommand(t, root, "kitten", `printf '%s\n' "$KITTY_CONFIG_DIRECTORY" "$@" > "$TEST_RECORD"
printf 'include current-theme.conf\n' > "$KITTY_CONFIG_DIRECTORY/theme.conf"
printf 'background #123456\n' > "$KITTY_CONFIG_DIRECTORY/current-theme.conf"
`))
	for _, name := range []string{"", "Solarized Dark"} {
		if err := Theme(name); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(record)
		if err != nil {
			t.Fatal(err)
		}
		want := filepath.Join(config, "local") + "\nthemes\n--config-file-name\ntheme.conf\n--reload-in\nparent\n"
		if name != "" {
			want += "--\n" + name + "\n"
		}
		if string(data) != want {
			t.Fatalf("picker invocation: %q", data)
		}
		if _, err := os.Stat(filepath.Join(config, "local", "current-theme.conf")); err != nil {
			t.Fatal(err)
		}
		if os.Getenv("KITTY_CONFIG_DIRECTORY") != config {
			t.Fatal("parent environment changed")
		}
	}
	if _, err := os.Stat(filepath.Join(config, "kitty.conf")); !os.IsNotExist(err) {
		t.Fatal("shared config was created")
	}
}

func TestThemeErrors(t *testing.T) {
	root := t.TempDir()
	t.Setenv("KITTY_REPO_ROOT", "")
	t.Setenv("KITTY_CONFIG_DIRECTORY", filepath.Join(root, "conf"))
	if err := Theme(""); err == nil || !strings.Contains(err.Error(), "kt instance") {
		t.Fatalf("missing instance: %v", err)
	}
	t.Setenv("KITTY_REPO_ROOT", root)
	t.Setenv("KITTY_CONFIG_DIRECTORY", filepath.Join(root, "global"))
	if err := Theme(""); err == nil {
		t.Fatal("unrelated config accepted")
	}
	t.Setenv("KITTY_CONFIG_DIRECTORY", filepath.Join(root, "conf"))
	t.Setenv("KITTY_REPO_KITTEN", fakeCommand(t, root, "kitten", "exit 7\n"))
	if err := Theme(""); err == nil {
		t.Fatal("picker failure ignored")
	}
}
