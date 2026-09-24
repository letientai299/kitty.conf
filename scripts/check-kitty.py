import os
import runpy
import time
from pathlib import Path
from types import SimpleNamespace

from kitty.config import load_config
from kitty.fast_data_types import Screen
from kitty.rgb import color_from_int
from kitty.tab_bar import as_rgb

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


def draw(tab_id, title, width, active=True, last=True, palette=None):
    screen = Screen(None, 1, 40)
    tab = SimpleNamespace(tab_id=tab_id, title=title, is_active=active)
    if palette is None:
        palette = (
            options.active_tab_foreground,
            options.active_tab_background,
            options.inactive_tab_foreground,
            options.inactive_tab_background,
            options.tab_bar_background or options.background,
        )
    fg, bg = palette[:2] if active else palette[2:4]
    screen.cursor.fg, screen.cursor.bg = as_rgb(int(fg)), as_rgb(int(bg))
    data = SimpleNamespace(default_bg=palette[4])
    end = draw_tab(data, screen, tab, 0, width, 1, last, None)
    assert end <= width, (title, width, end)
    assert screen.cursor.x == end and screen.cursor.y == 0
    for x in range(end):
        cell = screen.line(0).cursor_from(x)
        if x == 1:
            assert color_from_int(cell.fg >> 8).contrast(color_from_int(int(bg))) >= 3
        else:
            assert cell.fg == as_rgb(int(fg)), (x, cell.fg, fg)
        assert cell.bg == as_rgb(int(bg)), (x, cell.bg, bg)
    if last:
        for x in range(end, screen.columns):
            assert screen.line(0).cursor_from(x).bg == as_rgb(int(data.default_bg))
    return screen


profile = Screen(None, 1, 40).color_profile
marker_color = module["marker_color"]
for index in range(16):
    profile.set_color(index, 0x88AA00 + index)
identities = ("project", "another-project", "task", "manual title")
markers = [marker_color(name, profile, 0) for name in identities]
assert len(set(markers)) > 1
for index in range(16):
    profile.set_color(index, 0x88AA20 + index)
assert [marker_color(name, profile, 0) for name in identities] == [
    color + (0x20 << 8) for color in markers
]
for index in range(16):
    profile.set_color(index, 0x5588BB)
first = marker_color("project", profile, 0)
assert first == as_rgb(0x5588BB)
assert marker_color("project", profile, 0) == first
for index in range(16):
    profile.set_color(index, 0xBB8855)
assert marker_color("project", profile, 0) == as_rgb(0xBB8855)
for bg in (0, 0x5588BB, 0xFFFFFF):
    adjusted = marker_color("project", profile, bg)
    assert color_from_int(adjusted >> 8).contrast(color_from_int(bg)) >= 3


deadline = time.monotonic() + 5
while 1 not in worker.styles and time.monotonic() < deadline:
    draw(1, "zsh", 20)
    time.sleep(0.02)
assert worker.styles[1]["active"]["segments"][1]["text"] == "live", worker.styles
assert (state / "task-1").read_text() == "live"
assert not (state / "task-2").exists()
assert (state / "other").exists()

for palette in (
    (0xEEEEEE, 0x223344, 0xAAAAAA, 0x112233, 0x101010),
    (0x111111, 0xCCDDEE, 0x333333, 0xEEEEEE, 0xFFFFFF),
):
    for active in (False, True):
        for tab_id in (1, 2):
            draw(tab_id, "fallback", 20, active=active, palette=palette)

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
