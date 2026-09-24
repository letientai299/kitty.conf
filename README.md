# Kitty configuration

A standalone [Kitty][kitty] configuration with Go utilities and a custom tab bar.

```sh
mise install
mise run install
kt
```

`mise run install` links `~/.local/bin/kt` to this checkout’s `bin/kt`.
Keep this checkout in place and include `~/.local/bin` in `PATH`. Rebuilds
update the installed command.

The launcher opens a new instance using this checkout. Each instance gets a
private socket and task directory. Existing Kitty instances and the dotfiles
configuration keep their settings.

Kitty, `fzf`, and `nvim` must be installed. The default font is system monospace.
URL actions use `~/.local/bin/browse` when available, otherwise the OS opener. [mise][mise] manages development
tools; it does not install desktop applications. On macOS, `mise run qt:register`
binds Cmd+Esc to this checkout's quick terminal. `mise run qt:unregister` restores
the previous shortcut. These tasks explicitly change macOS Services settings.

Licensed under the [MIT license][license].

See the [guide][guide], [architecture][architecture], and
[development instructions][development]. Local font/theme settings and the
persistent-zoom proposal are in [configuration][configuration].

[kitty]: https://sw.kovidgoyal.net/kitty/
[mise]: https://mise.jdx.dev/
[guide]: docs/guide.md
[architecture]: docs/architecture.md
[development]: docs/development.md
[license]: LICENSE
[configuration]: docs/configuration.md
