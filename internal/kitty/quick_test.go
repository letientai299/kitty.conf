package kitty

import (
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestQuickToggle(t *testing.T) {
	state := filepath.Join(t.TempDir(), "s")
	if err := os.Mkdir(state, 0o700); err != nil {
		t.Fatal(err)
	}
	socket := filepath.Join(state, "rc-123")
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			_ = conn.Close()
		}
	}()
	root := t.TempDir()
	kitten := filepath.Join(root, "fake kitten")
	if err := os.WriteFile(kitten, []byte("#!/bin/sh\nenv > \"$KITTY_REPO_STATE/env\"\nprintf '%s\\n' \"$@\" > \"$KITTY_REPO_STATE/args\"\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	task := filepath.Join(state, "task-1")
	if err := os.WriteFile(task, []byte("keep task"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("KITTY_LISTEN_ON", "unix:/old")
	t.Setenv("KITTY_WINDOW_ID", "old-window")
	for range 2 {
		if err := quickTerminal(root, kitten, root, state, "test-group"); err != nil {
			t.Fatal(err)
		}
	}
	data, err := os.ReadFile(task)
	if err != nil || string(data) != "keep task" {
		t.Fatalf("task = %s, %v", data, err)
	}
	env, err := os.ReadFile(filepath.Join(state, "env"))
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"KITTY_CONFIG_DIRECTORY=" + filepath.Join(root, "conf"), "KITTY_REPO_STATE=" + state, "KITTY_REPO_ROOT=" + root} {
		if !strings.Contains(string(env), expected+"\n") {
			t.Fatalf("missing environment: %s", expected)
		}
	}
	if strings.Contains(string(env), "KITTY_LISTEN_ON=") || strings.Contains(string(env), "KITTY_WINDOW_ID=") {
		t.Fatal("inherited instance environment")
	}
	args, err := os.ReadFile(filepath.Join(state, "args"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(args), "kitty_conf="+filepath.Join(root, "conf", "kitty.conf")+"\n--override\nkitty_conf="+filepath.Join(root, "conf", "mac.conf")) {
		t.Fatalf("config ordering: %s", args)
	}
}

func TestQuickColdStart(t *testing.T) {
	state := filepath.Join(t.TempDir(), "s")
	if err := os.Mkdir(state, 0o700); err != nil {
		t.Fatal(err)
	}
	kitten := filepath.Join(state, "kitten")
	if err := os.WriteFile(kitten, []byte("#!/bin/sh\necho started > \"$KITTY_REPO_STATE/started\"\nwhile [ -d \"$KITTY_REPO_STATE\" ] && [ ! -f \"$KITTY_REPO_STATE/exit\" ]; do sleep 0.1; done\necho stopped > \"$KITTY_REPO_STATE/stopped\"\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.WriteFile(filepath.Join(state, "exit"), nil, 0o600)
		for range 100 {
			if _, err := os.Stat(filepath.Join(state, "stopped")); err == nil {
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
		t.Error("test child did not stop")
	})
	ready := make(chan net.Listener, 1)
	go func() {
		for range 100 {
			if _, err := os.Stat(filepath.Join(state, "started")); err == nil {
				listener, _ := net.Listen("unix", filepath.Join(state, "rc-1"))
				ready <- listener
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
		ready <- nil
	}()
	err := quickTerminal(state, kitten, state, state, "test-cold")
	listener := <-ready
	if listener != nil {
		t.Cleanup(func() { _ = listener.Close() })
	}
	if err != nil {
		t.Fatal(err)
	}
	if listener == nil {
		t.Fatal("socket creation failed")
	}
}

func TestQuickEarlyExit(t *testing.T) {
	state := filepath.Join(t.TempDir(), "s")
	err := quickTerminal(state, "/usr/bin/false", t.TempDir(), state, "test-exit")
	if err == nil || !strings.Contains(err.Error(), "quick terminal exited") {
		t.Fatalf("early exit: %v", err)
	}
}

func TestQuickState(t *testing.T) {
	state := filepath.Join(t.TempDir(), "s")
	if err := os.Mkdir(state, 0o700); err != nil {
		t.Fatal(err)
	}
	alive, err := quickAlive(filepath.Join(state, "rc"))
	if err != nil || alive {
		t.Fatalf("missing socket: %v, %v", alive, err)
	}
	for _, name := range []string{"task-1", "lock", "other"} {
		if err := os.WriteFile(filepath.Join(state, name), []byte("keep"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := clearQuickTasks(state); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(state, "task-1")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("stale task survived")
	}
	for _, name := range []string{"lock", "other"} {
		if _, err := os.Stat(filepath.Join(state, name)); err != nil {
			t.Fatal(err)
		}
	}
	public := filepath.Join(t.TempDir(), "public")
	if err := os.Mkdir(public, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := quickTerminal("unused", "unused", "unused", public, "unused"); err == nil {
		t.Fatal("accepted public state directory")
	}
}
