import glob
import os
import sys
import time
import unicodedata

from kitty.fast_data_types import get_boss, truncate_point_for_length, wcswidth
from kitty.tab_bar import TabAccessor, as_rgb

# Kitty's runpy loader omits sibling imports.
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from kitty_shared import find_git_root_and_dir, get_project_color, read_tab_task

_BORING_PROCESSES = frozenset(
    {"sh", "bash", "zsh", "fish", "dash", "ksh", "tcsh", "csh"}
)
_task_cache = {}
_tab_project_cache = {}
_TASK_FILE_PREFIX = os.path.join(os.environ["KITTY_REPO_STATE"], "task-")
_last_task_prune = 0


def _prune_task_files(boss):
    global _last_task_prune
    now = time.monotonic()
    if now - _last_task_prune < 5:
        return
    _last_task_prune = now
    live = {tab.id for tab in boss.all_tabs}
    for tab_id in _tab_project_cache.keys() - live:
        del _tab_project_cache[tab_id]
    for path in glob.glob(_TASK_FILE_PREFIX + "*"):
        suffix = path[len(_TASK_FILE_PREFIX) :]
        if suffix.isdigit() and int(suffix) not in live:
            try:
                os.remove(path)
            except OSError:
                pass


def _dim_color(color):
    return sum(int(((color >> shift) & 255) * 0.45) << shift for shift in (16, 8, 0))


def _get_project_name(cwd):
    now = time.monotonic()
    if cwd in _task_cache:
        expires, project = _task_cache[cwd]
        if now < expires:
            return project
    root, _ = find_git_root_and_dir(cwd)
    project = os.path.basename(root) if root else None
    if len(_task_cache) >= 64:
        del _task_cache[min(_task_cache, key=lambda key: _task_cache[key][0])]
    # Retry transient directories without rescanning every frame.
    _task_cache[cwd] = (now + (10 if project else 0.5), project)
    return project


def _process_name(accessor, title):
    process = accessor.active_exe or accessor.active_oldest_exe or title
    if os.path.basename(process).lstrip("-") in _BORING_PROCESSES:
        oldest = accessor.active_oldest_exe
        if oldest and os.path.basename(oldest).lstrip("-") not in _BORING_PROCESSES:
            process = oldest
        elif title.split() and title.split()[0] not in _BORING_PROCESSES:
            process = title.split()[0]
    return os.path.basename(process).lstrip("-")


def _segments(tab, boss, foreground):
    tab_obj = boss.tab_for_id(tab.tab_id)
    if tab_obj and tab_obj.name:
        return [(foreground, tab.title)]
    task = read_tab_task(tab.tab_id)
    if task:
        color = get_project_color(task)
        return [(color if tab.is_active else _dim_color(color), task)]
    accessor = TabAccessor(tab.tab_id)
    cwd = accessor.active_wd
    project = _get_project_name(cwd) if cwd else None
    if project:
        _tab_project_cache[tab.tab_id] = project
    else:
        project = _tab_project_cache.get(tab.tab_id)
    if project:
        brackets = 0x888888 if tab.is_active else 0x777777
        return [(brackets, "["), (get_project_color(project), project), (brackets, "]")]
    if not cwd:
        return [(foreground, tab.title)]
    home = os.path.expanduser("~")
    folder = "~" if cwd == home else os.path.basename(cwd.rstrip("/")) or "/"
    process = _process_name(accessor, tab.title)
    return [
        (0xD2FFD2 if tab.is_active else 0xB4DCB4, process + " "),
        (0xFFFFFF if tab.is_active else 0xC8C8C8, "\uf07c "),
        (0xFFE6B4 if tab.is_active else 0xDCC8A0, folder),
    ]


def _draw_segments(screen, segments, width):
    remaining = max(0, width)
    for color, text in segments:
        if remaining <= 0:
            break
        text = "".join(
            " " if unicodedata.category(char) == "Cc" else char for char in text
        )
        screen.cursor.fg = as_rgb(color)
        if wcswidth(text) > remaining:
            text = text[: truncate_point_for_length(text, remaining - 1)] + "…"
        screen.draw(text)
        remaining -= wcswidth(text)


def draw_tab(
    draw_data, screen, tab, before, max_title_length, index, is_last, extra_data
):
    boss = get_boss()
    _prune_task_files(boss)
    foreground = 0xFFFFFF if tab.is_active else 0x999999
    screen.cursor.bg = as_rgb(0x1A3A6A if tab.is_active else 0x2A2A2A)
    segments = [(foreground, " "), *_segments(tab, boss, foreground), (foreground, " ")]
    width = min(max_title_length, screen.columns - screen.cursor.x)
    _draw_segments(screen, segments, width)
    end = screen.cursor.x
    if is_last:
        screen.cursor.bg = as_rgb(0)
        screen.draw(" " * max(0, screen.columns - end))
        screen.cursor.x = end
    return end
