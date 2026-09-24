# Development

[mise][mise] pins tool versions and defines all tasks in `.config/mise.toml`.

Ruff keeps `ruff.toml` at the root for automatic CLI and editor discovery.

```sh
mise install
mise run hooks
mise run fmt
mise run check
```

[prek][prek] runs the repository checks before commits using `.config/prek.toml`.
Run hooks manually with `prek run --config .config/prek.toml --all-files`. Formatting covers Go,
Python, Markdown, and YAML. Linting covers Go and Python.
The check task builds the executable and runs tests with Go's race detector.
Python remains only for the Kitty callback and its tests; Kitty supplies its own
Python runtime in production.

Tests exercise project detection, instance task isolation, switcher rows,
launcher environment isolation, and the Python task reader/pruner. The launcher
integration test uses a fake Kitty process to inspect arguments and cleanup
without modifying a running terminal.

For a manual check, launch two instances, set different tasks on their first
tabs, then close one. Confirm the other label survives. Exercise the switcher,
preview, scrollback cursor placement, URL actions, tab reordering, and narrow
window rendering. These checks need a desktop session and the external tools
listed in the README.

The source checkout in dotfiles remains untouched. This repo deliberately
contains no installation task that replaces the global config symlink.

[mise]: https://mise.jdx.dev/
[prek]: https://prek.j178.dev/
