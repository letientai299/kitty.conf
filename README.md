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

Kitty, `fzf`, `nvim`, and Iosevka Nerd Font Mono must be installed. URL actions
use the existing `~/.local/bin/browse` helper. [mise][mise] manages development
tools; it does not install desktop applications or change global configuration.

See the [guide][guide], [architecture][architecture], and
[development instructions][development].

[kitty]: https://sw.kovidgoyal.net/kitty/
[mise]: https://mise.jdx.dev/
[guide]: docs/guide.md
[architecture]: docs/architecture.md
[development]: docs/development.md
