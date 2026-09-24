import hashlib
import json
import os
import select
import subprocess
import sys
import time
import unicodedata

import kitty.tab_bar as kitty_tab_bar
from kitty.fast_data_types import Color, get_boss, truncate_point_for_length, wcswidth
from kitty.rgb import color_from_int
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
    title = "".join(" " if unicodedata.category(c) == "Cc" else c for c in tab.title)
    return {"segments": [{"text": text} for text in (" ", title, " ")]}


def marker_color(name, profile, background):
    accents = (1, 2, 3, 4, 5, 6, 9, 10, 11, 12, 13, 14)
    slot = accents[
        int.from_bytes(hashlib.sha256(name.encode()).digest()) % len(accents)
    ]
    color = profile.as_color((slot << 8) | 1)
    assert color is not None
    bg = color_from_int(background)
    target = max((Color(0, 0, 0), Color(255, 255, 255)), key=bg.contrast)
    adjusted = color
    for step in range(1, 21):
        if adjusted.contrast(bg) >= 3:
            break
        amount = step / 20
        adjusted = Color(
            round(color.red + (target.red - color.red) * amount),
            round(color.green + (target.green - color.green) * amount),
            round(color.blue + (target.blue - color.blue) * amount),
        )
    return as_rgb(int(adjusted))


def draw_tab(
    draw_data, screen, tab, _before, max_title_length, _index, is_last, _extra_data
):
    worker.refresh(get_boss())
    styles = worker.styles.get(tab.tab_id)
    style = (
        styles["active" if tab.is_active else "inactive"] if styles else fallback(tab)
    )
    # Kitty initializes the cursor with theme colors.
    foreground = screen.cursor.fg
    segments = [(segment["text"], foreground) for segment in style["segments"]]
    name = "".join(text for text, _ in segments).strip()
    marker = marker_color(name, screen.color_profile, screen.cursor.bg >> 8)
    segments[:0] = [(" ", foreground), ("●", marker)]
    remaining = min(max_title_length, screen.columns - screen.cursor.x)
    for text, color in segments:
        if remaining <= 0:
            break
        if wcswidth(text) > remaining:
            text = text[: truncate_point_for_length(text, remaining - 1)] + "…"
        screen.cursor.fg = color
        screen.draw(text)
        remaining -= wcswidth(text)
    end = screen.cursor.x
    screen.cursor.fg = foreground
    if is_last:
        screen.cursor.bg = as_rgb(int(draw_data.default_bg))
        screen.draw(" " * max(0, screen.columns - end))
        screen.cursor.x = end
    return end
