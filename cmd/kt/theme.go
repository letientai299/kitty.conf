package main

import (
	"github.com/spf13/cobra"

	"kitty.conf/internal/kitty"
)

func newTheme() *cobra.Command {
	var pane, tab, window bool
	cmd := &cobra.Command{
		Use:   "theme [name]",
		Short: "Choose a Kitty theme",
		Long:  "Save a theme, or apply it temporarily to the current pane, tab, or OS window.\nOptionally select a theme by name without opening the picker.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			name := ""
			if len(args) == 1 {
				name = args[0]
			}
			scope := kitty.ThemeSaved
			switch {
			case pane:
				scope = kitty.ThemePane
			case tab:
				scope = kitty.ThemeTab
			case window:
				scope = kitty.ThemeWindow
			}
			return kitty.Theme(name, scope)
		},
	}
	cmd.Flags().BoolVar(&pane, "pane", false, "Change the current pane temporarily")
	cmd.Flags().BoolVar(&tab, "tab", false, "Change the current tab temporarily")
	cmd.Flags().BoolVar(&window, "window", false, "Change the current OS window temporarily")
	cmd.MarkFlagsMutuallyExclusive("pane", "tab", "window")
	return cmd
}
