package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestHelpHidesInternalCommands(t *testing.T) {
	root := newRoot()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"--help"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	for _, hidden := range []string{"preview", "pager", "stress"} {
		if strings.Contains(out.String(), "  "+hidden) {
			t.Errorf("%s listed:\n%s", hidden, out.String())
		}
	}
	for _, visible := range []string{"switch", "task", "remote", "completion"} {
		if !strings.Contains(out.String(), "  "+visible) {
			t.Errorf("%s missing:\n%s", visible, out.String())
		}
	}
}

func TestRootRejectsBareProgram(t *testing.T) {
	for _, args := range [][]string{{"zsh"}, {"launch"}, {"preview"}, {"preview", "x"}} {
		root := newRoot()
		root.SetArgs(args)
		if err := root.Execute(); err == nil {
			t.Errorf("accepted %v", args)
		}
	}
}
