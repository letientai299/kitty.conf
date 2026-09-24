package kitty

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
)

func Launch(args []string) error {
	binary, err := os.Executable()
	if err != nil {
		return err
	}
	binary, err = filepath.EvalSymlinks(binary)
	if err != nil {
		return err
	}
	root := filepath.Dir(filepath.Dir(binary))
	if _, err = os.Stat(filepath.Join(root, "conf", "kitty.conf")); err != nil {
		return err
	}
	kittyBin, err := exec.LookPath("kitty")
	if err != nil && runtime.GOOS == "darwin" {
		kittyBin = "/Applications/kitty.app/Contents/MacOS/kitty"
		_, err = os.Stat(kittyBin)
	}
	if err != nil {
		return err
	}
	real, err := filepath.EvalSymlinks(kittyBin)
	if err != nil {
		return err
	}
	kitten := filepath.Join(filepath.Dir(real), "kitten")
	if _, err = os.Stat(kitten); err != nil {
		kitten, err = exec.LookPath("kitten")
	}
	if err != nil {
		return err
	}
	// Keep Unix socket paths below macOS limits.
	state, err := os.MkdirTemp("/tmp", "kitty-conf-")
	if err != nil {
		return err
	}
	defer func() {
		if err := os.RemoveAll(state); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	}()
	argv := []string{"--config", filepath.Join(root, "conf", "kitty.conf"), "--listen-on", "unix:" + filepath.Join(state, "rc")}
	if runtime.GOOS == "darwin" {
		argv = append(argv, "--config", filepath.Join(root, "conf", "mac.conf"))
	}
	argv = append(argv, "--")
	argv = append(argv, args...)
	cmd := exec.Command(kittyBin, argv...)
	cmd.Env = launchEnv(os.Environ(), root, state, kitten)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	// Terminal interrupts belong to the launching shell.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)
	if err := cmd.Start(); err != nil {
		return err
	}
	done := make(chan struct{})
	defer close(done)
	go func() {
		for {
			select {
			case sig := <-signals:
				_ = cmd.Process.Signal(sig)
			case <-done:
				return
			}
		}
	}()
	return cmd.Wait()
}

func launchEnv(env []string, root, state, kitten string) []string {
	overrides := map[string]string{"KITTY_CONFIG_DIRECTORY": filepath.Join(root, "conf"), "KITTY_REPO_ROOT": root, "KITTY_REPO_STATE": state, "KITTY_REPO_KITTEN": kitten}
	var result []string
	for _, item := range env {
		key := item
		for i, c := range item {
			if c == '=' {
				key = item[:i]
				break
			}
		}
		if _, ok := overrides[key]; ok {
			continue
		}
		switch key {
		case "KITTY_LISTEN_ON", "KITTY_WINDOW_ID", "KITTY_PID", "KITTY_PUBLIC_KEY":
			continue
		}
		result = append(result, item)
	}
	for key, value := range overrides {
		result = append(result, key+"="+value)
	}
	return result
}
