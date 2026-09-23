package main

import (
	"fmt"
	"os"
	"strconv"

	"kitty.conf/internal/kitty"
)

func run(args []string) error {
	if len(args) == 0 {
		return kitty.Launch(nil)
	}
	switch args[0] {
	case "--help", "help":
		fmt.Println("Usage: kt [-- program [arguments]] | <command> [arguments]\nCommands: launch, switch, task, pager, ls, preview, stress, remote")
		return nil
	case "--", "launch":
		return kitty.Launch(args[1:])
	case "switch":
		return kitty.Switch()
	case "task":
		return kitty.EditTask()
	case "pager":
		return kitty.Pager(args[1:])
	case "stress":
		return kitty.Stress(args[1:])
	case "remote":
		return kitty.Broadcast()
	case "ls":
		data, err := kitty.Remote("ls")
		if err != nil {
			return err
		}
		_, err = os.Stdout.Write(data)
		return err
	case "preview":
		if len(args) != 2 {
			return fmt.Errorf("preview requires a window ID")
		}
		id, err := strconv.Atoi(args[1])
		if err != nil || id <= 0 {
			return fmt.Errorf("window ID is invalid")
		}
		data, err := kitty.Remote("get-text", "--match", kitty.Match(kitty.WindowID(id)), "--ansi", "--extent", "screen")
		if err != nil {
			return err
		}
		_, err = os.Stdout.Write(data)
		return err
	default:
		return fmt.Errorf("unknown command: %s", args[0])
	}
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
