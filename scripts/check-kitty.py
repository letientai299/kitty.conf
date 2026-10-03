import os
import runpy
import time
from pathlib import Path
from types import SimpleNamespace

from kitty.config import load_config
from kitty.fast_data_types import Screen
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
contrast_ratio = module["contrast_ratio"]
assert contrast_ratio(0, 0xFFFFFF) == 21
assert contrast_ratio(0xFFFFFF, 0) == 21
assert contrast_ratio(0x777777, 0x777777) == 1
assert abs(contrast_ratio(0x777777, 0xFFFFFF) - 4.478) < 0.001
assert abs(contrast_ratio(0xD2E0FF, 0x3A3C49) - 8.239) < 0.01
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


def draw(tab_id, title, width, active=True, last=True, palette=None, offset=0):
    screen = Screen(None, 1, 40)
    screen.cursor.x = offset
    tab = SimpleNamespace(tab_id=tab_id, os_window_id=1, title=title, is_active=active)
    if palette is None:
        palette = (
            options.active_tab_foreground,
            options.active_tab_background,
            options.inactive_tab_foreground,
            options.inactive_tab_background,
            options.tab_bar_background or options.background,
            options.color_table[4],
        )
    screen.color_profile.set_color(4, int(palette[5]))
    fg, bg = palette[:2] if active else palette[2:4]
    screen.cursor.fg, screen.cursor.bg = as_rgb(int(fg)), as_rgb(int(bg))
    data = SimpleNamespace(default_bg=palette[4], tab_bar_edge="bottom")
    end = draw_tab(data, screen, tab, offset, width, 1, last, None)
    available_width = min(width, screen.columns - offset)
    assert offset <= end <= offset + available_width, (title, width, offset, end)
    assert screen.cursor.x == end and screen.cursor.y == 0
    base = int(data.default_bg)
    for x in range(offset, end):
        cell = screen.line(0).cursor_from(x)
        edge = available_width >= 2 and x in (offset, end - 1)
        if edge:
            assert cell.fg == as_rgb(base)
        elif active:
            available = max(
                contrast_ratio(value, cell.bg >> 8) for value in (0, 0xFFFFFF)
            )
            assert contrast_ratio(cell.fg >> 8, cell.bg >> 8) >= min(7.0, available)
            assert cell.bold and not cell.italic
        else:
            assert cell.fg == as_rgb(0xFFFFFF)
            assert contrast_ratio(cell.fg >> 8, cell.bg >> 8) >= 4.5
            assert not cell.bold and not cell.italic
        if not active:
            assert all(
                ((cell.bg >> (shift + 8)) & 255) <= ((int(bg) >> shift) & 255)
                for shift in (16, 8, 0)
            )
    if available_width >= 2:
        text = str(screen.line(0)).rstrip()
        assert text[offset:].startswith("") and text.endswith(""), text
    if last:
        for x in range(end, screen.columns):
            assert screen.line(0).cursor_from(x).bg == as_rgb(int(data.default_bg))
    return screen


deadline = time.monotonic() + 5
while 1 not in worker.styles and time.monotonic() < deadline:
    draw(1, "zsh", 20)
    time.sleep(0.02)
assert worker.styles[1]["active"]["segments"][1]["text"] == "live", worker.styles
assert (state / "task-1").read_text() == "live"
assert not (state / "task-2").exists()
assert (state / "other").exists()

for palette in (
    (0xCDDBF9, 0xA8B6D0, 0xFFFFFF, 0x1A1A22, 0x20202A, 0xCDDBF9),
    (0xEEEEEE, 0x223344, 0xAAAAAA, 0x112233, 0x101010, 0x46BDFF),
    (0x111111, 0xCCDDEE, 0x333333, 0xEEEEEE, 0xFFFFFF, 0x224488),
):
    for active in (False, True):
        for tab_id in (1, 2):
            draw(tab_id, "fallback", 20, active=active, palette=palette)

for title in ("任务" * 40, "ascii" * 40, "é" * 40, "bad\n\x1btitle"):
    for width in (0, 1, 2, 3, 8, 20):
        draw(1, title, width)
        draw(2, title, width, active=False, last=False)
        for offset in (33, 39):
            draw(2, title, width, active=False, offset=offset)
            draw(1, title, width, offset=offset)

dark_palette = (0xCDDBF9, 0xA8B6D0, 0xFFFFFF, 0x1A1A22, 0x20202A, 0xCDDBF9)
bright = draw(2, "contrast", 20, palette=dark_palette)
assert bright.line(0).cursor_from(2).fg >> 8 == 0xD2E0FF
changed = draw(2, "contrast", 20, palette=dark_palette[:5] + (0xFF6600,))
assert changed.line(0).cursor_from(2).fg != bright.line(0).cursor_from(2).fg

worker.stop()
assert worker.proc is None
globals_["_BINARY"] = str(state / "missing")
broken = module["Worker"]()
globals_["worker"] = broken
screen = draw(1, "plain", 20)
assert broken.proc is None and broken.styles == {}
print("Kitty config and callback checks passed.")
