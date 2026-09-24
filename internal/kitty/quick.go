package kitty

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"
)

func QuickTerminal() error {
	if runtime.GOOS != "darwin" {
		return errors.New("quick terminal requires macOS")
	}
	root, err := checkoutRoot()
	if err != nil {
		return err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	// GUI services do not inherit shell PATH.
	kitten := "/Applications/kitty.app/Contents/MacOS/kitten"
	if _, err = os.Stat(kitten); err != nil {
		kitten = filepath.Join(home, "Applications", "kitty.app", "Contents", "MacOS", "kitten")
		if _, err = os.Stat(kitten); err != nil {
			return err
		}
	}
	id := fmt.Sprintf("%x", sha256.Sum256([]byte(root)))[:16]
	state := fmt.Sprintf("/tmp/kitty-qt-%d-%s", os.Getuid(), id)
	return quickTerminal(root, kitten, home, state, "kitty-conf-"+id)
}

func quickTerminal(root, kitten, home, state, group string) (result error) {
	if err := os.Mkdir(state, 0o700); err != nil && !errors.Is(err, os.ErrExist) {
		return err
	}
	info, err := os.Lstat(state)
	if err != nil {
		return err
	}
	owner, ok := info.Sys().(*syscall.Stat_t)
	if !ok || !info.IsDir() || info.Mode().Perm() != 0o700 || int64(owner.Uid) != int64(os.Getuid()) {
		return errors.New("quick terminal state ownership is invalid")
	}
	fd, err := syscall.Open(filepath.Join(state, "lock"), syscall.O_CREAT|syscall.O_RDWR|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0o600)
	if err != nil {
		return err
	}
	lock := os.NewFile(uintptr(fd), "quick terminal lock")
	defer func() { result = errors.Join(result, lock.Close()) }()
	if err := syscall.Flock(fd, syscall.LOCK_EX); err != nil {
		return err
	}
	socket := filepath.Join(state, "rc")
	alive, err := quickAlive(socket)
	if err != nil {
		return err
	}
	if !alive {
		if err := clearQuickTasks(state); err != nil {
			return err
		}
	}
	cmd := exec.Command(kitten, quickArgs(root, socket, group)...)
	cmd.Env = launchEnv(os.Environ(), root, state, kitten)
	cmd.Dir = home
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if alive {
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}
	logPath := filepath.Join(state, "startup.log")
	log, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, log.Close()) }()
	cmd.Stdout, cmd.Stderr = log, log
	if err := cmd.Start(); err != nil {
		return err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case err := <-done:
			return errors.Join(fmt.Errorf("quick terminal exited; inspect %s", logPath), err)
		default:
		}
		if alive, err = quickAlive(socket); err != nil || alive {
			return err
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("quick terminal socket startup timed out; inspect %s", logPath)
}

func quickArgs(root, socket, group string) []string {
	return []string{
		"quick-access-terminal", "--instance-group", group,
		"--config", filepath.Join(root, "conf", "quick-access-terminal.conf"),
		"--override", "kitty_conf=" + filepath.Join(root, "conf", "kitty.conf"),
		"--override", "kitty_conf=" + filepath.Join(root, "conf", "mac.conf"),
		"--override", "kitty_override=listen_on=unix:" + socket,
		"--override", "kitty_override=allow_remote_control=socket-only",
	}
}

func quickAlive(socket string) (bool, error) {
	// Configured Kitty sockets receive a PID suffix.
	paths, err := filepath.Glob(socket + "-*")
	if err != nil {
		return false, err
	}
	active := 0
	for _, path := range paths {
		conn, err := net.DialTimeout("unix", path, 250*time.Millisecond)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) || errors.Is(err, syscall.ECONNREFUSED) {
				continue
			}
			return false, err
		}
		if err := conn.Close(); err != nil {
			return false, err
		}
		active++
	}
	if active > 1 {
		return false, errors.New("multiple quick terminal sockets are active")
	}
	return active == 1, nil
}

func clearQuickTasks(state string) error {
	entries, err := os.ReadDir(state)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "task-") && !entry.IsDir() {
			if err := os.Remove(filepath.Join(state, entry.Name())); err != nil {
				return err
			}
		}
	}
	return nil
}
