package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"

	"kitty.conf/internal/kitty"
	"kitty.conf/internal/tabbar"
)

func newRoot() *cobra.Command {
	root := &cobra.Command{
		Use:   "kt [-- program [arguments]]",
		Short: "Isolated Kitty instances and pane tools",
		Long: `kt launches an isolated Kitty instance in the current directory.
Pass a program after -- to run it instead of a shell.`,
		Args:          cobra.ArbitraryArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 && cmd.ArgsLenAtDash() != 0 {
				return fmt.Errorf("unknown command: %s", args[0])
			}
			return kitty.Launch(args)
		},
	}
	root.AddCommand(
		&cobra.Command{
			Use:                "__launch",
			Hidden:             true,
			DisableFlagParsing: true,
			RunE: func(_ *cobra.Command, args []string) error {
				if len(args) > 0 && args[0] == "--" {
					args = args[1:]
				}
				return kitty.LaunchWorker(args, os.NewFile(3, "launch status"))
			},
		},
		&cobra.Command{
			Use:   "qt",
			Short: "Toggle this checkout's quick terminal",
			Args:  cobra.NoArgs,
			RunE:  func(*cobra.Command, []string) error { return kitty.QuickTerminal() },
		},
		&cobra.Command{
			Use:   "switch",
			Short: "Pick a pane with fzf and focus it",
			Args:  cobra.NoArgs,
			RunE:  func(*cobra.Command, []string) error { return kitty.Switch() },
		},
		&cobra.Command{
			Use:   "task",
			Short: "Edit the current tab's task label",
			Args:  cobra.NoArgs,
			RunE:  func(*cobra.Command, []string) error { return kitty.EditTask() },
		},
		newRemote(),
		newTheme(),
		&cobra.Command{
			Use:    "theme-preview <pane-id> <name>",
			Hidden: true,
			Args:   cobra.ExactArgs(2),
			RunE: func(_ *cobra.Command, args []string) error {
				id, err := strconv.Atoi(args[0])
				if err != nil || id <= 0 {
					return errors.New("preview pane ID is invalid")
				}
				return kitty.PreviewTheme(args[1], kitty.WindowID(id))
			},
		},
		&cobra.Command{
			Use:   "ls",
			Short: "Print the instance layout as JSON",
			Args:  cobra.NoArgs,
			RunE: func(*cobra.Command, []string) error {
				return writeRemote("ls")
			},
		},
		&cobra.Command{
			Use:    "preview <window-id>",
			Short:  "Print a window's screen for fzf previews",
			Hidden: true,
			Args:   cobra.ExactArgs(1),
			RunE: func(_ *cobra.Command, args []string) error {
				id, err := strconv.Atoi(args[0])
				if err != nil || id <= 0 {
					return errors.New("window ID is invalid")
				}
				return writeRemote("get-text", "--match", kitty.Match(kitty.WindowID(id)), "--ansi", "--extent", "screen")
			},
		},
		&cobra.Command{
			Use:    "pager [INPUT_LINE_NUMBER CURSOR_LINE CURSOR_COLUMN]",
			Short:  "Open scrollback from stdin in Neovim",
			Hidden: true,
			Args:   cobra.ArbitraryArgs,
			RunE:   func(_ *cobra.Command, args []string) error { return kitty.Pager(args) },
		},
		newTabBar(),
		&cobra.Command{
			Use:                "stress {cpu [workers] | memory [mb [seconds]] | network {download|upload|both} URL...}",
			Short:              "Generate load for terminal testing",
			Hidden:             true,
			DisableFlagParsing: true,
			Args:               cobra.ArbitraryArgs,
			RunE:               func(_ *cobra.Command, args []string) error { return kitty.Stress(args) },
		},
	)
	return root
}

func newRemote() *cobra.Command {
	var opts kitty.BroadcastOptions
	cmd := &cobra.Command{
		Use:   "remote",
		Short: "Broadcast typed input to several panes",
		Long: `Broadcast keyboard input to panes through Kitty's broadcast kitten.
Targets every pane in the current tab unless --pick selects them.`,
		Args: cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error { return kitty.Broadcast(opts) },
	}
	cmd.Flags().IntVarP(&opts.New, "new", "n", 0, "create this many panes first")
	cmd.Flags().BoolVarP(&opts.Pick, "pick", "p", false, "choose target panes with fzf")
	return cmd
}

func newTabBar() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "tab-bar",
		Short:  "Compute tab-bar labels for conf/tab_bar.py",
		Hidden: true,
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "render",
		Short: "Render one JSON snapshot from stdin",
		Args:  cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			var req tabbar.Request
			if err := json.NewDecoder(os.Stdin).Decode(&req); err != nil {
				return fmt.Errorf("snapshot decode failed: %w", err)
			}
			out, err := tabbar.NewBuilder().Render(req)
			if err != nil {
				return err
			}
			return json.NewEncoder(os.Stdout).Encode(out)
		},
	}, &cobra.Command{
		Use:   "serve",
		Short: "Answer JSON snapshots on stdin until EOF",
		Args:  cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			return tabbar.Serve(os.Stdin, os.Stdout)
		},
	})
	return cmd
}

func writeRemote(args ...string) error {
	data, err := kitty.Remote(args...)
	if err != nil {
		return err
	}
	_, err = os.Stdout.Write(data)
	return err
}

func main() {
	if err := newRoot().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
