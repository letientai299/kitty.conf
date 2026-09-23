import os
import runpy
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
state = Path(os.environ["KITTY_REPO_STATE"])
(state / "task-1").write_text("live")
(state / "task-2").write_text("closed")
(state / "other").write_text("unrelated")
module["_prune_task_files"](SimpleNamespace(all_tabs=[SimpleNamespace(id=1)]))
assert (state / "task-1").read_text() == "live"
assert not (state / "task-2").exists()
assert (state / "other").exists()
print("Kitty config and callback checks passed.")


globals_ = module["draw_tab"].__globals__
boss = SimpleNamespace(
    all_tabs=[SimpleNamespace(id=1)],
    tab_for_id=lambda _: SimpleNamespace(name="manual"),
)
globals_["get_boss"] = lambda: boss
for title in ("任务" * 40, "ascii" * 40, "e\u0301" * 40, "bad\n\x1btitle"):
    for width in (0, 1, 2, 3, 8, 20):
        screen = Screen(None, 1, 40)
        tab = SimpleNamespace(tab_id=1, title=title, is_active=True)
        end = module["draw_tab"](None, screen, tab, 0, width, 1, True, None)
        assert end <= width, (title, width, end)
        assert screen.cursor.x == end
        assert screen.cursor.y == 0

globals_["_tab_project_cache"][999] = "closed"
globals_["_last_task_prune"] = 0
module["_prune_task_files"](boss)
assert 999 not in globals_["_tab_project_cache"]

(state / "task-1").unlink()
globals_["TabAccessor"] = lambda _: SimpleNamespace(
    active_wd=str(root), active_exe="zsh", active_oldest_exe="zsh"
)
boss.tab_for_id = lambda _: SimpleNamespace(name="")
for label in ("任务" * 40, " " * 10):
    tab = SimpleNamespace(tab_id=1, title=label, is_active=True)
    for width in (0, 1, 3, 8, 20):
        screen = Screen(None, 1, 40)
        end = module["draw_tab"](None, screen, tab, 0, width, 1, False, None)
        assert end <= width
assert module["_process_name"](globals_["TabAccessor"](1), "   ") == "zsh"
