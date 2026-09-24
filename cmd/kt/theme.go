package main

import (
	"github.com/spf13/cobra"

	"kitty.conf/internal/kitty"
)

func newTheme() *cobra.Command {
	return &cobra.Command{
		Use:   "theme [name]",
		Short: "Choose and save a local Kitty theme",
		Long:  "Open the theme picker inside a kt instance.\nOptionally select a theme by name without opening the picker.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			name := ""
			if len(args) == 1 {
				name = args[0]
			}
			return kitty.Theme(name)
		},
	}
}
