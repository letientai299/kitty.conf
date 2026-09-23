# Architecture

The launcher selects this checkout through `KITTY_CONFIG_DIRECTORY` and explicit
config paths. It creates a private temporary directory for the instance socket
and task files, clears inherited instance identifiers, and runs Kitty without
single-instance reuse. Closing Kitty removes that directory. Forced termination
of the launcher can leave an unused directory under `/tmp/kitty-conf-*`.

The Go executable provides short-lived commands for the switcher, task editor,
scrollback pager, diagnostics, and stress tools. It uses the standard library
and Kitty's native `kitten @` client. Remote calls require `KITTY_LISTEN_ON`;
they never guess a global socket. A daemon adds no benefit to these on-demand
operations and would require lifecycle and recovery machinery.

Kitty requires a Python `draw_tab` callback for [custom tab rendering][tabs].
`conf/tab_bar.py` therefore stays in-process, with `conf/kitty_shared.py` providing cached
project lookup support and task-file reads. Drawing starts no helper processes.
Task reads cache file contents by inode, timestamps, and size. Project misses
retry after half a second; closed tabs release their project cache entries.
Switcher project lookups are cached for one invocation.

Go and Python retain the same MD5-based palette mapping for visual continuity;
MD5 serves only as a color selector. Task files live under `KITTY_REPO_STATE`,
so one instance cannot prune another instance's labels.

The scrollback pager streams input to a temporary file, opens Neovim with a
terminal buffer, then removes the file. Coordinates are parsed as integers and
the file path reaches Neovim through an environment variable, avoiding command
interpolation. The switcher strips control characters from display fields and
uses numeric IDs for preview and focus commands.

The [Kitty invocation reference][invocation] defines configuration-directory
selection and socket inheritance. The `BenchmarkRows` benchmark measures switcher row construction.
Full GUI startup and interactive latency have not been benchmarked.

[tabs]: https://sw.kovidgoyal.net/kitty/conf/#opt-kitty.tab_bar_style
[invocation]: https://sw.kovidgoyal.net/kitty/invocation/
