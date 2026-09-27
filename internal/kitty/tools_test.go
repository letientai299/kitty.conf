package kitty

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMiseShims(t *testing.T) {
	for _, tt := range []struct{ name, shims, data, xdg, want string }{
		{name: "default", want: ".local/share/mise/shims"},
		{name: "xdg", xdg: "data", want: "data/mise/shims"},
		{name: "mise", data: "mise", xdg: "data", want: "mise/shims"},
		{name: "override", shims: "custom", data: "mise", want: "custom"},
		{name: "tilde", shims: "~/custom", want: "custom"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			for key, value := range map[string]string{"MISE_SHIMS_DIR": tt.shims, "MISE_DATA_DIR": tt.data, "XDG_DATA_HOME": tt.xdg} {
				if value != "" && !strings.HasPrefix(value, "~/") {
					value = filepath.Join(home, value)
				}
				t.Setenv(key, value)
			}
			if got, want := miseShims(), filepath.Join(home, tt.want); got != want {
				t.Fatalf("got %q, want %q", got, want)
			}
		})
	}
}

func TestToolLookup(t *testing.T) {
	for _, tt := range []struct {
		name          string
		shim, global  bool
		mode          os.FileMode
		want, warning string
	}{
		{"mise first", true, true, 0700, "mise", ""},
		{"mise without PATH", true, false, 0700, "mise", ""},
		{"PATH fallback", false, true, 0700, "global", "warning:"},
		{"non executable shim", true, true, 0600, "global", "warning:"},
		{"missing", false, false, 0700, "", ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			shims, global := t.TempDir(), t.TempDir()
			t.Setenv("MISE_SHIMS_DIR", shims)
			t.Setenv("PATH", global)
			if tt.shim {
				path := fakeCommand(t, shims, "fzf", "printf mise")
				if err := os.Chmod(path, tt.mode); err != nil {
					t.Fatal(err)
				}
			}
			if tt.global {
				fakeCommand(t, global, "fzf", "printf global")
			}
			reader, writer, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			original := os.Stderr
			os.Stderr = writer
			t.Cleanup(func() { os.Stderr = original; _ = writer.Close(); _ = reader.Close() })
			cmd := toolCommand("fzf")
			os.Stderr = original
			_ = writer.Close()
			warning, err := io.ReadAll(reader)
			if err != nil {
				t.Fatal(err)
			}
			if (tt.warning == "" && len(warning) != 0) || (tt.warning != "" && !strings.Contains(string(warning), tt.warning)) {
				t.Fatalf("warning: %q", warning)
			}
			output, err := cmd.Output()
			if tt.want == "" {
				if err == nil || !strings.Contains(err.Error(), "required tool fzf missing from mise shims and PATH") {
					t.Fatalf("missing tool: %v", err)
				}
			} else if err != nil || string(output) != tt.want {
				t.Fatalf("output: %q, %v", output, err)
			}
		})
	}
}

func TestExplicitTool(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("MISE_SHIMS_DIR", dir)
	fakeCommand(t, dir, "kitten", "printf mise")
	explicit := fakeCommand(t, t.TempDir(), "kitten", "printf explicit")
	output, err := toolCommand(explicit).Output()
	if err != nil || string(output) != "explicit" {
		t.Fatalf("explicit binary: %q, %v", output, err)
	}
	if err := toolCommand(filepath.Join(dir, "absent")).Run(); err == nil {
		t.Fatal("missing explicit binary accepted")
	}
}
