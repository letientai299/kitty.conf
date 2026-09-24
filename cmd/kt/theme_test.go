package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestThemeCommand(t *testing.T) {
	root := newRoot()
	var output bytes.Buffer
	root.SetOut(&output)
	root.SetArgs([]string{"theme", "--help"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "theme [name]") {
		t.Fatalf("theme command missing: %s", output.String())
	}
	root = newRoot()
	root.SetArgs([]string{"theme", "one", "two"})
	if err := root.Execute(); err == nil {
		t.Fatal("extra theme arguments accepted")
	}
}
