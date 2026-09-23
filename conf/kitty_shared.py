import hashlib
import os
from functools import lru_cache

PROJECT_COLORS = (
    0x50FA7B,
    0xFF79C6,
    0x8BE9FD,
    0xFFB86C,
    0xBD93F9,
    0xF1FA8C,
    0xFF6E6E,
    0x69FF94,
    0xA4FFFF,
    0xFF92DF,
    0xFFD580,
    0xCAA9FA,
)


@lru_cache(maxsize=256)
def get_project_color(name):
    digest = int.from_bytes(hashlib.md5(name.encode()).digest())
    return PROJECT_COLORS[digest % len(PROJECT_COLORS)]


def tab_task_file(tab_id):
    return os.path.join(os.environ["KITTY_REPO_STATE"], f"task-{tab_id}")


@lru_cache(maxsize=128)
def _read_task(path, signature):
    with open(path, encoding="utf-8", errors="replace") as file:
        for line in file:
            if line.strip():
                return line.strip()
    return None


def read_tab_task(tab_id):
    try:
        path = tab_task_file(tab_id)
        info = os.stat(path)
        return _read_task(
            path, (info.st_ino, info.st_mtime_ns, info.st_ctime_ns, info.st_size)
        )
    except OSError:
        return None


def find_git_root_and_dir(cwd):
    if not cwd:
        return None, None
    path = os.path.abspath(cwd)
    while True:
        git_path = os.path.join(path, ".git")
        if os.path.isdir(git_path):
            return path, git_path
        if os.path.isfile(git_path):
            try:
                with open(git_path, encoding="utf-8", errors="replace") as file:
                    line = file.readline().strip()
                if line.startswith("gitdir: "):
                    git_dir = line[8:]
                    if not os.path.isabs(git_dir):
                        git_dir = os.path.normpath(os.path.join(path, git_dir))
                    return path, git_dir
            except OSError:
                pass
            return None, None
        parent = os.path.dirname(path)
        if parent == path:
            return None, None
        path = parent
