# Guide

Run `kt` from any working directory after `mise run install`. The working
directory becomes the first shell's directory. Keep the launcher running until
that instance closes; its process owns task-file cleanup.

To run a program instead of a shell:

```sh
kt -- nvim
```

`mise run launch` builds before opening Kitty. `conf/kitty.conf` contains shared
settings; the launcher adds `conf/mac.conf` on macOS. No Ubuntu-specific layer exists.
Changing files takes effect on the next launch or Kitty's reload shortcut.

| Shortcut                  | Action                                        |
| ------------------------- | --------------------------------------------- |
| Ctrl+Shift+F              | Search windows, tabs, and panes with previews |
| Alt+F1                    | Edit the current tab's task in Neovim         |
| Alt+1 through Alt+9       | Select a tab                                  |
| Ctrl+Shift+T / Cmd+T      | New tab in the current directory              |
| Ctrl+Shift+Enter          | New pane in the current directory             |
| Cmd+Enter                 | New pane on macOS                             |
| Alt+H/J/K/L               | Navigate panes                                |
| Ctrl+Shift+L, S/T/G/F/V/H | Select a layout                               |
| Ctrl+Shift+L, L           | Cycle layouts                                 |
| Ctrl+Shift+Alt+H/J/K/L    | Resize panes                                  |
| Ctrl+Shift+Z              | Toggle maximization                           |
| Ctrl+Shift+H              | Open scrollback in Neovim                     |
| Ctrl+Shift+O              | Copy the previous command's output            |
| Alt+F2                    | Detach the pane into a new tab                |
| Ctrl+Shift+E              | Open URL hints through `browse`               |

A task uses the first nonempty, trimmed line of its file. Clear the file to
restore automatic project labels. Tasks follow stable tab IDs across pane
changes and tab reordering. They expire when the tab or instance closes. Manual
Kitty tab titles take precedence in the tab bar. The switcher preserves its
original task/project fallback behavior.

The tab bar retains deterministic project/task colors, inactive dimming,
process/folder fallback, and project caching. The active theme and automatic
no-preference theme are copied from dotfiles. The unused third-party theme
catalog stays in dotfiles; use Kitty's themes kitten to choose another theme.
The quick-access settings remain in `conf/quick-access-terminal.conf` for Kitty's
quick-access facility; `kt` opens a normal terminal.

`mise run install` links `kt` into `~/.local/bin` and writes zsh completion to
`~/.local/share/zsh/site-functions/_kt`. `kt --help` lists the commands meant
for interactive use; `kt completion` covers other shells.

`kt remote` broadcasts typed input to several panes through Kitty's broadcast
kitten. It targets every pane in the current tab. `--new N` creates N panes
first and switches the tab to grid layout; `--pick` selects target panes with
fzf across the whole instance. It works in any Kitty with remote control
enabled, including the host instance.

Hidden diagnostics run from the launched instance:

```sh
"$KITTY_REPO_ROOT/bin/kt" ls
"$KITTY_REPO_ROOT/bin/kt" stress cpu 4
"$KITTY_REPO_ROOT/bin/kt" stress memory 64 0.2
"$KITTY_REPO_ROOT/bin/kt" stress network both DOWNLOAD_URL UPLOAD_URL
```

Stress commands stop on Ctrl+C. Memory stress grows until stopped. Network stress
requires explicit endpoints; upload payload size comes from `UPLOAD_MB` and
defaults to 16 MiB. No stress workload runs during normal startup.
