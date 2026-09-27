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
		if err := Theme(name, ThemeSaved); err != nil {
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
	if err := Theme("", ThemeSaved); err == nil || !strings.Contains(err.Error(), "kt instance") {
		t.Fatalf("missing instance: %v", err)
	}
	t.Setenv("KITTY_REPO_ROOT", root)
	t.Setenv("KITTY_CONFIG_DIRECTORY", filepath.Join(root, "global"))
	if err := Theme("", ThemeSaved); err == nil {
		t.Fatal("unrelated config accepted")
	}
	t.Setenv("KITTY_CONFIG_DIRECTORY", filepath.Join(root, "conf"))
	t.Setenv("KITTY_REPO_KITTEN", fakeCommand(t, root, "kitten", "exit 7\n"))
	if err := Theme("", ThemeSaved); err == nil {
		t.Fatal("picker failure ignored")
	}
}

func TestTemporaryTheme(t *testing.T) {
	root := t.TempDir()
	record := filepath.Join(root, "record")
	t.Setenv("KITTY_REPO_ROOT", root)
	t.Setenv("KITTY_CONFIG_DIRECTORY", filepath.Join(root, "conf"))
	t.Setenv("KITTY_LISTEN_ON", "unix:/kitty")
	t.Setenv("KITTY_WINDOW_ID", "7")
	t.Setenv("TEST_RECORD", record)
	t.Setenv("KITTY_REPO_KITTEN", fakeCommand(t, root, "kitten", `if [ "$1" = @ ] && [ "$4" = ls ]; then
 printf '%s' '[{"tabs":[{"windows":[{"id":7},{"id":8}]},{"windows":[{"id":9}]}]},{"tabs":[{"windows":[{"id":10}]}]}]'
 exit
fi
printf '%s\n' "$@" >> "$TEST_RECORD"
if [ "$1" = themes ]; then
 printf 'background #123456\n'
fi
`))
	for _, tt := range []struct {
		scope ThemeScope
		match string
	}{
		{ThemePane, "--match\nid:7\n"},
		{ThemeTab, "--match-tab\nwindow_id:7\n"},
		{ThemeWindow, "--match\nid:7 or id:8 or id:9\n"},
	} {
		if err := os.RemoveAll(record); err != nil {
			t.Fatal(err)
		}
		if err := Theme("Carbonfox", tt.scope); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(record)
		if err != nil {
			t.Fatal(err)
		}
		text := string(data)
		if !strings.Contains(text, "themes\n--dump-theme\n--\nCarbonfox\n") || !strings.Contains(text, tt.match) || !strings.Contains(text, "set-colors\n") {
			t.Fatalf("scope %s: %s", tt.scope, text)
		}
		for line := range strings.SplitSeq(text, "\n") {
			if strings.HasPrefix(filepath.Base(line), "kt-theme-") {
				if _, err := os.Stat(line); !os.IsNotExist(err) {
					t.Fatal("temporary theme file remains")
				}
			}
		}
		if _, err := os.Stat(filepath.Join(root, "conf", "local")); !os.IsNotExist(err) {
			t.Fatal("temporary theme wrote local config")
		}
	}
}

func TestTemporaryThemeCancel(t *testing.T) {
	root := t.TempDir()
	record := filepath.Join(root, "record")
	t.Setenv("KITTY_REPO_ROOT", root)
	t.Setenv("KITTY_CONFIG_DIRECTORY", filepath.Join(root, "conf"))
	t.Setenv("KITTY_LISTEN_ON", "unix:/kitty")
	t.Setenv("KITTY_WINDOW_ID", "7")
	t.Setenv("TEST_RECORD", record)
	t.Setenv("KITTY_REPO_KITTEN", fakeCommand(t, root, "kitten", `if [ "${4:-}" = ls ]; then
 printf '[{"tabs":[{"id":3,"layout":"grid","windows":[{"id":7}]}]}]'
elif [ "${4:-}" = launch ]; then
 printf '12\n'
elif [ "$1" = __complete__ ]; then
 printf '%s\n' "$@" >> "$TEST_RECORD"
 printf '[{"groups":[{"matches":[{"word":"3024 Night"},{"word":"Carbonfox"}]}]}]'
else
 printf '%s\n' "$@" >> "$TEST_RECORD"
fi
`))
	fakeCommand(t, root, "fzf", "cat >/dev/null\nexit 130\n")
	t.Setenv("MISE_SHIMS_DIR", t.TempDir())
	t.Setenv("PATH", root+string(os.PathListSeparator)+os.Getenv("PATH"))
	if err := Theme("", ThemePane); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(record)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "set-colors") || !strings.Contains(string(data), "close-window\n--match\nid:12\n") {
		t.Fatal("cancel changed colors or left preview")
	}
	if err := Theme("Carbonfox", ThemePane); err == nil {
		t.Fatal("empty palette accepted")
	}
	t.Setenv("KITTY_REPO_KITTEN", fakeCommand(t, root, "kitten", `printf '# default theme\n'`))
	if err := Theme("Default", ThemePane); err == nil || !strings.Contains(err.Error(), "no colors") {
		t.Fatalf("empty palette: %v", err)
	}
	t.Setenv("KITTY_WINDOW_ID", "")
	if err := Theme("", ThemePane); err == nil {
		t.Fatal("missing pane accepted")
	}
	t.Setenv("KITTY_WINDOW_ID", "7")
	t.Setenv("KITTY_LISTEN_ON", "")
	if err := Theme("", ThemePane); err == nil {
		t.Fatal("missing socket accepted")
	}
}

func TestTemporaryThemePicker(t *testing.T) {
	root := t.TempDir()
	record := filepath.Join(root, "record")
	t.Setenv("KITTY_REPO_ROOT", root)
	t.Setenv("KITTY_CONFIG_DIRECTORY", filepath.Join(root, "conf"))
	t.Setenv("KITTY_LISTEN_ON", "unix:/kitty")
	t.Setenv("KITTY_WINDOW_ID", "7")
	t.Setenv("TEST_RECORD", record)
	t.Setenv("MISE_SHIMS_DIR", t.TempDir())
	t.Setenv("PATH", root+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("KITTY_REPO_KITTEN", fakeCommand(t, root, "kitten", `printf '%s\n' "$@" >> "$TEST_RECORD"
if [ "$1" = __complete__ ]; then
 cat > "$TEST_RECORD.input"
 printf '[{"groups":[{"matches":[{"word":"3024 Night"},{"word":"Carbonfox"}]}]}]'
fi
if [ "$1" = themes ]; then printf 'background #123456\n'; fi
if [ "${4:-}" = ls ]; then printf '[{"tabs":[{"id":3,"layout":"grid","windows":[{"id":7}]}]}]'; fi
if [ "${4:-}" = launch ]; then printf '12\n'; fi
`))
	fakeCommand(t, root, "fzf", `printf '%s\n' "$@" > "$TEST_RECORD.fzf"
cat > "$TEST_RECORD.names"
printf '3024 Night\n'
`)
	if err := Theme("", ThemePane); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(record)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "__complete__\njson\n") || !strings.Contains(string(data), "themes\n--dump-theme\n--\n3024 Night\n") || !strings.Contains(string(data), "set-colors\n") {
		t.Fatalf("picker invocation: %s", data)
	}
	input, err := os.ReadFile(record + ".input")
	if err != nil || string(input) != `[["kitten","themes",""]]` {
		t.Fatalf("completion input: %q, %v", input, err)
	}
	names, err := os.ReadFile(record + ".names")
	if err != nil || string(names) != "3024 Night\nCarbonfox\n" {
		t.Fatalf("picker names: %q, %v", names, err)
	}
	options, err := os.ReadFile(record + ".fzf")
	if err != nil || !strings.Contains(string(options), "focus:execute-silent(") || !strings.Contains(string(options), " theme-preview 12 {})") {
		t.Fatalf("picker preview: %q, %v", options, err)
	}
	if !strings.Contains(string(data), "launch\n--type=window\n--match\nid:3\n--dont-take-focus\n") || !strings.Contains(string(data), "close-window\n--match\nid:12\n") {
		t.Fatalf("preview pane lifecycle: %s", data)
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "--cwd\n"+cwd+"\n") || !strings.Contains(string(data), "/zsh\n-i\n-c\n") || !strings.Contains(string(data), "ls -lah\n") || !strings.Contains(string(data), "# The quick brown fox") || !strings.Contains(string(data), "# Dim comment:") || !strings.Contains(string(data), "exec zsh -i\n") {
		t.Fatalf("preview content: %s", data)
	}
}

func TestTemporaryThemePreviewCleanupStack(t *testing.T) {
	root := t.TempDir()
	record := filepath.Join(root, "record")
	t.Setenv("KITTY_REPO_ROOT", root)
	t.Setenv("KITTY_CONFIG_DIRECTORY", filepath.Join(root, "conf"))
	t.Setenv("KITTY_LISTEN_ON", "unix:/kitty")
	t.Setenv("KITTY_WINDOW_ID", "7")
	t.Setenv("TEST_RECORD", record)
	t.Setenv("MISE_SHIMS_DIR", t.TempDir())
	t.Setenv("PATH", root+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("KITTY_REPO_KITTEN", fakeCommand(t, root, "kitten", `printf '%s\n' "$@" >> "$TEST_RECORD"
case "${4:-}" in
 ls) printf '%s' '[{"tabs":[{"id":3,"layout":"stack","windows":[{"id":7},{"id":8}]}]}]' ;;
 launch) printf '12\n' ;;
esac
if [ "$1" = __complete__ ]; then
 printf '[{"groups":[{"matches":[{"word":"Carbonfox"}]}]}]'
fi
`))
	fakeCommand(t, root, "fzf", "cat >/dev/null\nexit 130\n")
	if err := Theme("", ThemeTab); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(record)
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	if !strings.Contains(got, "goto-layout\n--match\nid:3\ntall\n") || !strings.Contains(got, "close-window\n--match\nid:12\n--ignore-no-match\n") || !strings.Contains(got, "goto-layout\n--match\nid:3\nstack\n") || strings.Contains(got, "set-colors") {
		t.Fatalf("preview cleanup: %s", got)
	}
}

func TestPreviewThemeReusesPane(t *testing.T) {
	root := t.TempDir()
	record := filepath.Join(root, "record")
	t.Setenv("KITTY_REPO_ROOT", root)
	t.Setenv("KITTY_CONFIG_DIRECTORY", filepath.Join(root, "conf"))
	t.Setenv("KITTY_LISTEN_ON", "unix:/kitty")
	t.Setenv("TEST_RECORD", record)
	t.Setenv("KITTY_REPO_KITTEN", fakeCommand(t, root, "kitten", `if [ "${4:-}" = get-colors ]; then
 printf 'background #101010\nforeground #eeeeee'
elif [ "$1" = themes ]; then
 printf 'background #%s\n' "$4"
elif [ "${4:-}" = set-colors ]; then
 printf '%s\n' "$6" >> "$TEST_RECORD.targets"
 cp "$7" "$TEST_RECORD.palette"
fi
`))
	for _, name := range []string{"111111", "222222"} {
		if err := PreviewTheme(name, 12); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(record + ".palette")
		if err != nil || string(data) != "background #101010\nforeground #eeeeee\nbackground #"+name+"\n" {
			t.Fatalf("preview %s: %q, %v", name, data, err)
		}
	}
	targets, err := os.ReadFile(record + ".targets")
	if err != nil || string(targets) != "id:12\nid:12\n" {
		t.Fatalf("preview targets: %q, %v", targets, err)
	}
}
