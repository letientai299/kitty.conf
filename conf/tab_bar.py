import json
import os
import select
import subprocess
import sys
import time
import unicodedata
from functools import lru_cache

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
    title = "".join(" " if unicodedata.category(c) == "Cc" else c for c in tab.title)
    return {"segments": [{"text": text} for text in (" ", title, " ")]}


def contrast_ratio(foreground: int, background: int) -> float:
    def luminance(rgb: int) -> float:
        channels = [((rgb >> shift) & 255) / 255 for shift in (16, 8, 0)]
        linear = [
            value / 12.92 if value <= 0.04045 else ((value + 0.055) / 1.055) ** 2.4
            for value in channels
        ]
        return sum(
            value * weight for value, weight in zip(linear, (0.2126, 0.7152, 0.0722))
        )

    light, dark = sorted((luminance(foreground), luminance(background)), reverse=True)
    return (light + 0.05) / (dark + 0.05)


def blend_colors(first: int, second: int, amount: float) -> int:
    return sum(
        round(
            ((first >> shift) & 255) * (1 - amount) + ((second >> shift) & 255) * amount
        )
        << shift
        for shift in (16, 8, 0)
    )


@lru_cache(maxsize=128)
def tab_colors(active: bool, accent: int, base: int, inactive: int) -> tuple[int, int]:
    if not active:
        background = blend_colors(0, inactive, 0.75)
        while contrast_ratio(0xFFFFFF, background) < 4.5:
            background = blend_colors(0, background, 0.9)
        return 0xFFFFFF, background
    background = accent  # blend_colors(base, accent, 1.45)
    channels = [(accent >> shift) & 255 for shift in (16, 8, 0)]
    scale = 255 / max(1, max(channels))
    vivid = sum(
        round(channel * scale) << shift for channel, shift in zip(channels, (16, 8, 0))
    )
    target = max((0, 0xFFFFFF), key=lambda value: contrast_ratio(value, background))
    contrast = min(7.0, contrast_ratio(target, background))
    foreground = vivid
    for step in range(1, 21):
        if contrast_ratio(foreground, background) >= contrast:
            break
        foreground = blend_colors(vivid, target, step / 20)
    return foreground, background


def draw_tab(
    draw_data, screen, tab, _before, max_title_length, _index, is_last, _extra_data
):
    boss = get_boss()
    worker.refresh(boss)
    styles = worker.styles.get(tab.tab_id)
    style = (
        styles["active" if tab.is_active else "inactive"] if styles else fallback(tab)
    )
    foreground, background = screen.cursor.fg, screen.cursor.bg
    accent = screen.color_profile.as_color((4 << 8) | 1)
    accent_rgb = int(accent) if accent is not None else foreground >> 8
    base = int(draw_data.default_bg)
    label_rgb, rgb = tab_colors(tab.is_active, accent_rgb, base, background >> 8)
    screen.cursor.fg = as_rgb(label_rgb)
    screen.cursor.bg = as_rgb(rgb)
    screen.cursor.bold, screen.cursor.italic = tab.is_active, False
    remaining = max(0, min(max_title_length, screen.columns - screen.cursor.x))
    slants = remaining >= 2
    left, right = "", ""
    if slants:
        screen.cursor.fg = as_rgb(base)
        screen.draw(left)
        screen.cursor.fg = as_rgb(label_rgb)
        remaining -= 2
    for segment in style["segments"]:
        if remaining <= 0:
            break
        text = segment["text"]
        if wcswidth(text) > remaining:
            text = text[: truncate_point_for_length(text, remaining - 1)] + "…"
        screen.draw(text)
        remaining -= wcswidth(text)
    if slants:
        screen.cursor.fg = as_rgb(base)
        screen.draw(right)
    end = screen.cursor.x
    screen.cursor.fg, screen.cursor.bg = foreground, background
    if is_last:
        screen.cursor.bg = as_rgb(int(draw_data.default_bg))
        screen.draw(" " * max(0, screen.columns - end))
        screen.cursor.x = end
    return end
