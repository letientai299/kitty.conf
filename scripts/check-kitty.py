import os
import runpy
import time
from pathlib import Path
from types import SimpleNamespace

from kitty.config import load_config
from kitty.fast_data_types import Screen

root = Path(os.environ["KITTY_CONFIG_DIRECTORY"])
bad_lines = []
options = load_config(
    str(root / "kitty.conf"), str(root / "mac.conf"), accumulate_bad_lines=bad_lines
)
assert not bad_lines, bad_lines
assert options.tab_bar_style == "custom"
assert options.scrollback_pager[0] == str(root.parent / "bin/kt")
assert options.scrollback_pager[1] == "pager"
assert options.macos_option_as_alt

module = runpy.run_path(str(root / "tab_bar.py"))
draw_tab = module["draw_tab"]
globals_ = draw_tab.__globals__
worker = module["Worker"]()
globals_["worker"] = worker
state = Path(os.environ["KITTY_REPO_STATE"])
(state / "task-1").write_text("live")
(state / "task-2").write_text("closed")
(state / "other").write_text("unrelated")

tabs = [SimpleNamespace(id=1, os_window_id=1, name="", title="zsh")]
boss = SimpleNamespace(all_tabs=tabs)
globals_["get_boss"] = lambda: boss
globals_["TabAccessor"] = lambda _: SimpleNamespace(
    active_wd=str(root), active_exe="zsh", active_oldest_exe="zsh"
)


def draw(tab_id, title, width, active=True, last=True):
    screen = Screen(None, 1, 40)
    tab = SimpleNamespace(tab_id=tab_id, title=title, is_active=active)
    end = draw_tab(None, screen, tab, 0, width, 1, last, None)
    assert end <= width, (title, width, end)
    assert screen.cursor.x == end and screen.cursor.y == 0
    return screen


deadline = time.monotonic() + 5
while 1 not in worker.styles and time.monotonic() < deadline:
    draw(1, "zsh", 20)
    time.sleep(0.02)
assert worker.styles[1]["active"]["segments"][1]["text"] == "live", worker.styles
assert (state / "task-1").read_text() == "live"
assert not (state / "task-2").exists()
assert (state / "other").exists()

for title in ("任务" * 40, "ascii" * 40, "é" * 40, "bad\n\x1btitle"):
    for width in (0, 1, 2, 3, 8, 20):
        draw(1, title, width)
        draw(2, title, width, active=False, last=False)

worker.stop()
assert worker.proc is None
globals_["_BINARY"] = str(state / "missing")
broken = module["Worker"]()
globals_["worker"] = broken
screen = draw(1, "plain", 20)
assert broken.proc is None and broken.styles == {}
print("Kitty config and callback checks passed.")
