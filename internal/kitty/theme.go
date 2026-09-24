package kitty

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

type ThemeScope string

const (
	ThemeSaved  ThemeScope = ""
	ThemePane   ThemeScope = "pane"
	ThemeTab    ThemeScope = "tab"
	ThemeWindow ThemeScope = "window"
)

func Theme(name string, scope ThemeScope) error {
	local, err := themeLocal()
	if err != nil {
		return err
	}
	match, err := themeMatch(scope)
	if err != nil {
		return err
	}
	if scope != ThemeSaved {
		if name == "" {
			name, err = pickTheme(local)
			if err != nil || name == "" {
				return err
			}
		}
		return applyNamedTheme(name, local, match)
	}
	if err := os.MkdirAll(local, 0o700); err != nil {
		return err
	}
	args := []string{"themes", "--config-file-name", "theme.conf", "--reload-in", "parent"}
	if name != "" {
		args = append(args, "--", name)
	}
	cmd := exec.Command(kittenBinary(), args...)
	cmd.Env = append(os.Environ(), "KITTY_CONFIG_DIRECTORY="+local)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd.Run()
}

func themeLocal() (string, error) {
	root := os.Getenv("KITTY_REPO_ROOT")
	config := os.Getenv("KITTY_CONFIG_DIRECTORY")
	if root == "" || config == "" || filepath.Clean(config) != filepath.Join(root, "conf") {
		return "", errors.New("theme requires a kt instance")
	}
	return filepath.Join(config, "local"), nil
}

func PreviewTheme(name string, id WindowID) error {
	if id <= 0 {
		return errors.New("preview pane ID is invalid")
	}
	local, err := themeLocal()
	if err != nil {
		return err
	}
	base, err := Remote("get-colors", "--configured")
	if err != nil {
		return err
	}
	if !themeHasColors(base) {
		return errors.New("configured colors are missing")
	}
	data, err := dumpTheme(name, local)
	if err != nil {
		return err
	}
	combined := make([]byte, 0, len(base)+1+len(data))
	combined = append(combined, base...)
	combined = append(combined, '\n')
	combined = append(combined, data...)
	return applyTheme(combined, []string{"--match", Match(id)})
}

func applyNamedTheme(name, local string, match []string) error {
	data, err := dumpTheme(name, local)
	if err != nil {
		return err
	}
	return applyTheme(data, match)
}

func dumpTheme(name, local string) ([]byte, error) {
	cmd := exec.Command(kittenBinary(), "themes", "--dump-theme", "--", name)
	cmd.Env = append(os.Environ(), "KITTY_CONFIG_DIRECTORY="+local)
	cmd.Stderr = os.Stderr
	data, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	if !themeHasColors(data) {
		return nil, errors.New("theme has no colors")
	}
	return data, nil
}

func applyTheme(data []byte, match []string) error {
	file, err := os.CreateTemp("", "kt-theme-*.conf")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(file.Name()) }()
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	_, err = Remote("set-colors", match[0], match[1], file.Name())
	return err
}

func pickTheme(local string) (name string, result error) {
	cmd := exec.Command(kittenBinary(), "__complete__", "json")
	cmd.Env = append(os.Environ(), "KITTY_CONFIG_DIRECTORY="+local)
	cmd.Stdin = strings.NewReader(`[["kitten","themes",""]]`)
	cmd.Stderr = os.Stderr
	data, err := cmd.Output()
	if err != nil {
		return "", err
	}
	var completion []struct {
		Groups []struct {
			Matches []struct {
				Word string `json:"word"`
			} `json:"matches"`
		} `json:"groups"`
	}
	if err := json.Unmarshal(data, &completion); err != nil {
		return "", err
	}
	var names []string
	for _, item := range completion {
		for _, group := range item.Groups {
			for _, match := range group.Matches {
				if match.Word != "" && match.Word != "Default" && !strings.ContainsAny(match.Word, "\r\n") {
					names = append(names, match.Word)
				}
			}
		}
	}
	if len(names) == 0 {
		return "", errors.New("no themes found")
	}
	slices.Sort(names)
	preview, err := startThemePreview()
	if err != nil {
		return "", err
	}
	defer func() { result = errors.Join(result, preview.close()) }()
	binary, err := os.Executable()
	if err != nil {
		return "", err
	}
	quoted := "'" + strings.ReplaceAll(binary, "'", "'\"'\"'") + "'"
	cmd = exec.Command("fzf", "--layout=reverse", "--border", "--prompt", "Theme > ", "--bind", "focus:execute-silent("+quoted+" theme-preview "+strconv.Itoa(int(preview.id))+" {})")
	cmd.Env = append(os.Environ(), "SHELL=/bin/sh", "FZF_DEFAULT_OPTS=", "FZF_DEFAULT_OPTS_FILE=")
	cmd.Stdin = strings.NewReader(strings.Join(names, "\n") + "\n")
	cmd.Stderr = os.Stderr
	selected, err := cmd.Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) && (exit.ExitCode() == 1 || exit.ExitCode() == 130) {
			return "", nil
		}
		return "", err
	}
	name = strings.TrimSuffix(string(selected), "\n")
	if !slices.Contains(names, name) {
		return "", errors.New("selected theme is invalid")
	}
	return name, nil
}

type themePreview struct {
	id     WindowID
	tab    TabID
	layout string
}

func startThemePreview() (themePreview, error) {
	id, err := strconv.Atoi(os.Getenv("KITTY_WINDOW_ID"))
	if err != nil || id <= 0 {
		return themePreview{}, errors.New("KITTY_WINDOW_ID is invalid")
	}
	windows, err := List()
	if err != nil {
		return themePreview{}, err
	}
	preview := themePreview{}
	for _, osWindow := range windows {
		for _, tab := range osWindow.Tabs {
			for _, pane := range tab.Windows {
				if int(pane.ID) == id {
					preview.tab = tab.ID
					if tab.Layout == "stack" {
						preview.layout = tab.Layout
					}
				}
			}
		}
	}
	if preview.tab <= 0 {
		return themePreview{}, errors.New("theme tab is missing")
	}
	cwd, err := os.Getwd()
	if err != nil {
		return themePreview{}, err
	}
	shell := os.Getenv("SHELL")
	if filepath.Base(shell) != "zsh" {
		shell, err = exec.LookPath("zsh")
		if err != nil {
			return themePreview{}, err
		}
	}
	tabMatch := "id:" + strconv.Itoa(int(preview.tab))
	if preview.layout != "" {
		if _, err := Remote("goto-layout", "--match", tabMatch, "tall"); err != nil {
			return themePreview{}, err
		}
	}
	const sample = `printf '\033[1mTheme preview\033[0m\n\n'
printf '$ ls -lah\n'
ls -lah
printf '\n# The quick brown fox jumps over the lazy dog while this long comment wraps across the preview pane.\n'
printf '\033[2m# Dim comment: /usr/local/share/example-project/configuration/theme.conf stays readable with this palette.\033[0m\n\n'
printf 'Normal  \033[1mBold\033[0m  \033[3mItalic\033[0m\n\n'
for n in 0 1 2 3 4 5 6 7; do printf '\033[4%sm  \033[0m' "$n"; done
printf '\n'
for n in 0 1 2 3 4 5 6 7; do printf '\033[10%sm  \033[0m' "$n"; done
printf '\n\nPress Enter in fzf to apply.\n'
exec zsh -i`
	data, err := Remote("launch", "--type=window", "--match", tabMatch, "--dont-take-focus", "--title", "Theme preview", "--location", "after", "--cwd", cwd, shell, "-i", "-c", sample)
	if err != nil {
		return themePreview{}, errors.Join(err, preview.close())
	}
	parsed, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || parsed <= 0 {
		return themePreview{}, errors.Join(errors.New("preview pane ID is invalid"), preview.close())
	}
	preview.id = WindowID(parsed)
	return preview, nil
}

func (preview themePreview) close() error {
	var errs []error
	if preview.id > 0 {
		_, err := Remote("close-window", "--match", Match(preview.id), "--ignore-no-match")
		errs = append(errs, err)
	}
	if preview.layout != "" {
		_, err := Remote("goto-layout", "--match", "id:"+strconv.Itoa(int(preview.tab)), preview.layout)
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

func themeHasColors(data []byte) bool {
	for line := range strings.SplitSeq(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			return true
		}
	}
	return false
}

func themeMatch(scope ThemeScope) ([]string, error) {
	if scope == ThemeSaved {
		return nil, nil
	}
	if os.Getenv("KITTY_LISTEN_ON") == "" {
		return nil, errors.New("KITTY_LISTEN_ON is missing")
	}
	id, err := strconv.Atoi(os.Getenv("KITTY_WINDOW_ID"))
	if err != nil || id <= 0 {
		return nil, errors.New("KITTY_WINDOW_ID is invalid")
	}
	switch scope {
	case ThemePane:
		return []string{"--match", Match(WindowID(id))}, nil
	case ThemeTab:
		return []string{"--match-tab", "window_id:" + strconv.Itoa(id)}, nil
	case ThemeWindow:
		windows, err := List()
		if err != nil {
			return nil, err
		}
		for _, osWindow := range windows {
			var ids []string
			found := false
			for _, tab := range osWindow.Tabs {
				for _, pane := range tab.Windows {
					ids = append(ids, Match(pane.ID))
					found = found || int(pane.ID) == id
				}
			}
			if found {
				return []string{"--match", strings.Join(ids, " or ")}, nil
			}
		}
		return nil, fmt.Errorf("window %d is missing", id)
	default:
		return nil, fmt.Errorf("theme scope %q is invalid", scope)
	}
}
