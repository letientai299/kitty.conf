package kitty

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
)

func Pager(args []string) error {
	cursor := "normal! G"
	if len(args) != 0 && len(args) != 3 {
		return errors.New("pager requires zero or three coordinates")
	}
	if len(args) == 3 {
		coords := [3]int{}
		for i, arg := range args {
			n, err := strconv.Atoi(arg)
			if err != nil || n < 0 {
				return fmt.Errorf("pager coordinate is invalid: %q", arg)
			}
			coords[i] = n
		}
		cursor = fmt.Sprintf("call cursor(%d, %d)", max(0, coords[0]-1)+max(1, coords[1]), max(1, coords[2]))
	}
	file, err := os.CreateTemp("", "kitty-scrollback-*")
	if err != nil {
		return err
	}
	defer func() {
		if err := os.Remove(file.Name()); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	}()
	_, copyErr := io.Copy(file, os.Stdin)
	err = file.Close()
	if copyErr != nil {
		return copyErr
	}
	if err != nil {
		return err
	}
	cmd := exec.Command("nvim",
		"-c", "set nonumber laststatus=0 colorcolumn=0 tw=10000",
		"-c", "autocmd TermEnter * stopinsert",
		"-c", "autocmd TermClose * "+cursor,
		"-c", "nnoremap <silent> q :qa!<CR>",
		"-c", `lua vim.fn.termopen({'/bin/cat', vim.env.KITTY_SCROLLBACK_FILE})`)
	cmd.Env = append(os.Environ(), "NVIM_PAGER=1", "KITTY_SCROLLBACK_FILE="+file.Name())
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer func() {
		if err := tty.Close(); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	}()
	cmd.Stdin, cmd.Stdout, cmd.Stderr = tty, tty, tty
	return cmd.Run()
}
