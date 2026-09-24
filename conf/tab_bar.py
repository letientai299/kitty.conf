import json
import os
import select
import subprocess
import sys
import time
import unicodedata

import kitty.tab_bar as kitty_tab_bar
from kitty.fast_data_types import get_boss, truncate_point_for_length, wcswidth
from kitty.tab_bar import TabAccessor, as_rgb

PROTOCOL = 1
REPLY_TIMEOUT = 0.002
SNAPSHOT_INTERVAL = 0.02
RESTART_BACKOFF = 5.0
_BINARY = os.path.join(os.environ["KITTY_REPO_ROOT"], "bin", "kt")


class Worker:
    """One `kt tab-bar serve` process; at most one request in flight."""

    def __init__(self):
        self.proc = None
        self.styles = {}
        self.revision = 0
        self.pending = False
        self.snapshot_at = self.retry_at = 0.0

    def stop(self, error=None):
        if error:
            print(f"tab_bar worker: {error}", file=sys.stderr)
        if self.proc:
            self.proc.kill()
            self.proc.wait()
        self.proc, self.pending = None, False

    def start(self):
        try:
            self.proc = subprocess.Popen(
                [_BINARY, "tab-bar", "serve"],
                stdin=subprocess.PIPE,
                stdout=subprocess.PIPE,
            )
        except OSError as error:
            self.stop(error)
        return self.proc

    def refresh(self, boss):
        now = time.monotonic()
        if now - self.snapshot_at < SNAPSHOT_INTERVAL:
            return
        self.snapshot_at = now
        proc = self.proc
        if proc is None:
            if now < self.retry_at:
                return
            self.retry_at = now + RESTART_BACKOFF
            proc = self.start()
        if proc is None or proc.stdin is None or proc.stdout is None:
            return
        try:
            if self.pending and not self.read(proc.stdout):
                return
            self.revision += 1
            request = {
                "version": PROTOCOL,
                "revision": self.revision,
                "complete": True,
                "tabs": [snapshot(tab) for tab in boss.all_tabs],
            }
            proc.stdin.write(json.dumps(request).encode() + b"\n")
            proc.stdin.flush()
            self.pending = True
            self.read(proc.stdout)
        except (OSError, ValueError) as error:
            self.stop(error)

    def read(self, stdout):
        """Apply a reply if one arrives in time. Returns True when idle."""
        if not select.select([stdout], [], [], REPLY_TIMEOUT)[0]:
            return False
        reply = json.loads(stdout.readline() or b"null")
        self.pending = False
        if not reply or reply.get("version") != PROTOCOL or reply.get("error"):
            raise ValueError(reply and (reply.get("error") or "protocol mismatch"))
        if reply["revision"] == self.revision:
            self.styles = {int(key): value for key, value in reply["tabs"].items()}
        return True


def snapshot(tab):
    accessor = TabAccessor(tab.id)
    return {
        "id": tab.id,
        "os_window_id": tab.os_window_id,
        "manual_title": tab.name or "",
        "title": tab.title or "",
        "cwd": accessor.active_wd or "",
        "active_exe": accessor.active_exe or "",
        "oldest_exe": accessor.active_oldest_exe or "",
    }


# Config reload re-runs this module; replace the worker so rebuilt binaries load.
previous = getattr(kitty_tab_bar, "_kitty_conf_worker", None)
if previous is not None:
    previous.stop()
worker = Worker()
setattr(kitty_tab_bar, "_kitty_conf_worker", worker)


def fallback(tab):
    fg = 0xFFFFFF if tab.is_active else 0x999999
    title = "".join(" " if unicodedata.category(c) == "Cc" else c for c in tab.title)
    segments = [{"text": text, "color": fg} for text in (" ", title, " ")]
    return {"background": 0x1A3A6A if tab.is_active else 0x2A2A2A, "segments": segments}


def draw_tab(
    _draw_data, screen, tab, _before, max_title_length, _index, is_last, _extra_data
):
    worker.refresh(get_boss())
    styles = worker.styles.get(tab.tab_id)
    style = (
        styles["active" if tab.is_active else "inactive"] if styles else fallback(tab)
    )
    screen.cursor.bg = as_rgb(style["background"])
    remaining = min(max_title_length, screen.columns - screen.cursor.x)
    for segment in style["segments"]:
        if remaining <= 0:
            break
        text = segment["text"]
        if wcswidth(text) > remaining:
            text = text[: truncate_point_for_length(text, remaining - 1)] + "…"
        screen.cursor.fg = as_rgb(segment["color"])
        screen.draw(text)
        remaining -= wcswidth(text)
    end = screen.cursor.x
    if is_last:
        screen.cursor.bg = as_rgb(0)
        screen.draw(" " * max(0, screen.columns - end))
        screen.cursor.x = end
    return end
