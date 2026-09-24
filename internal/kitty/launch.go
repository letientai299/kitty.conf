package kitty

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
)

func launchPaths() (root, kittyBin, kitten string, err error) {
	root, err = checkoutRoot()
	if err != nil {
		return
	}
	kittyBin, err = exec.LookPath("kitty")
	if err != nil && runtime.GOOS == "darwin" {
		kittyBin = "/Applications/kitty.app/Contents/MacOS/kitty"
		_, err = os.Stat(kittyBin)
	}
	if err != nil {
		return
	}
	real, err := filepath.EvalSymlinks(kittyBin)
	if err != nil {
		return
	}
	kitten = filepath.Join(filepath.Dir(real), "kitten")
	if _, err = os.Stat(kitten); err != nil {
		kitten, err = exec.LookPath("kitten")
	}
	return
}

func checkoutRoot() (string, error) {
	binary, err := os.Executable()
	if err != nil {
		return "", err
	}
	binary, err = filepath.EvalSymlinks(binary)
	if err != nil {
		return "", err
	}
	root := filepath.Dir(filepath.Dir(binary))
	if _, err = os.Stat(filepath.Join(root, "conf", "kitty.conf")); err != nil {
		return "", err
	}
	return root, nil
}

func Launch(args []string) error {
	binary, err := os.Executable()
	if err != nil {
		return err
	}
	reader, writer, err := os.Pipe()
	if err != nil {
		return err
	}
	defer func() { _ = reader.Close() }()
	cmd := exec.Command(binary, append([]string{"__launch", "--"}, args...)...)
	cmd.ExtraFiles = []*os.File{writer}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	cmd.Stdin, cmd.Stdout, cmd.Stderr = nil, nil, nil
	if err := cmd.Start(); err != nil {
		_ = writer.Close()
		return err
	}
	_ = writer.Close()
	defer func() { _ = cmd.Process.Release() }()
	status, err := io.ReadAll(reader)
	if err != nil {
		return err
	}
	if string(status) != "ok" {
		return fmt.Errorf("kitty launch failed: %s", status)
	}
	return nil
}

func LaunchWorker(args []string, ready *os.File) (result error) {
	syscall.CloseOnExec(int(ready.Fd()))
	defer func() {
		if ready == nil {
			return
		}
		if result != nil {
			_, _ = fmt.Fprint(ready, result)
		}
		_ = ready.Close()
	}()
	root, kittyBin, kitten, err := launchPaths()
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
	if _, err := ready.WriteString("ok"); err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return err
	}
	_ = ready.Close()
	ready = nil
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
