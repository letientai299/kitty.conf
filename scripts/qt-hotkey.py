import argparse
import fcntl
import json
import os
import shutil
import subprocess
import sys
import time
from importlib.util import module_from_spec, spec_from_file_location
from pathlib import Path

BEGIN = "-- >>> managed by kitty.conf quick terminal >>>"
END = "-- <<< managed by kitty.conf quick terminal <<<"
VARIABLE = "KITTY_CONF_QT_HOTKEY"


def service_module(root):
    spec = spec_from_file_location("qt_service", root / "scripts/qt-service.py")
    assert spec and spec.loader
    module = module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def block(root):
    script = json.dumps(str(root / "scripts/qt-hotkey.lua"), ensure_ascii=False)
    checkout = json.dumps(str(root), ensure_ascii=False)
    return (
        f"{BEGIN}\n"
        f"{VARIABLE} = assert(loadfile({script}))({checkout})\n"
        f"assert({VARIABLE}, 'quick terminal hotkey unavailable')\n"
        f"{END}\n"
    )


class Hotkey:
    def __init__(self, root, home, command=None):
        self.root = root.resolve()
        self.path = home / ".hammerspoon/init.lua"
        self.command = command or shutil.which("hs")

    def content(self):
        if self.path.is_symlink() or not self.path.is_file():
            raise RuntimeError("Hammerspoon config is missing or a symlink")
        content = self.path.read_text()
        expected = block(self.root)
        if content.count(BEGIN) != content.count(END):
            raise RuntimeError("Quick terminal hotkey markers are invalid")
        if content.count(BEGIN) > 1:
            raise RuntimeError("Quick terminal hotkey markers are duplicated")
        if BEGIN in content and expected not in content:
            raise RuntimeError("Quick terminal hotkey changed after registration")
        return content

    def write(self, content):
        mode = self.path.stat().st_mode & 0o777
        service_module(self.root).atomic_write(self.path, content.encode())
        self.path.chmod(mode)

    def loaded(self, expected):
        if not self.command:
            raise RuntimeError("Hammerspoon CLI is missing")
        deadline = time.monotonic() + 5
        while time.monotonic() < deadline:
            result = subprocess.run(
                [self.command, "-c", f"return {VARIABLE} ~= nil"],
                capture_output=True,
                text=True,
            )
            if (
                result.returncode == 0
                and result.stdout.strip() == str(expected).lower()
            ):
                return
            time.sleep(0.1)
        raise RuntimeError("Hammerspoon hotkey reload failed")

    def reload(self, expected):
        if not self.command:
            raise RuntimeError("Hammerspoon CLI is missing")
        subprocess.run([self.command, "-c", "hs.reload()"], capture_output=True)
        self.loaded(expected)

    def register(self):
        content = self.content()
        expected = block(self.root)
        if expected in content:
            self.loaded(True)
            return False
        self.write(content + ("" if content.endswith("\n") else "\n") + expected)
        try:
            self.reload(True)
        except Exception:
            self.write(content)
            self.reload(False)
            raise
        return True

    def unregister(self):
        if not self.path.exists() and not self.path.is_symlink():
            return
        content = self.content()
        expected = block(self.root)
        if expected not in content:
            return
        self.write(content.replace(expected, "", 1))
        self.reload(False)


def main():
    parser = argparse.ArgumentParser(description="Manage the quick terminal hotkey")
    parser.add_argument("action", choices=("register", "unregister"))
    args = parser.parse_args()
    if sys.platform != "darwin":
        raise RuntimeError("Quick terminal hotkey requires macOS")
    root = Path(__file__).resolve().parents[1]
    home = Path.home()
    if args.action == "register" and not os.access(root / "bin/kt", os.X_OK):
        raise RuntimeError("Quick terminal executable is missing")
    module = service_module(root)
    service = module.Registration(root, home, module.Preferences())
    hotkey = Hotkey(root, home)
    service.directory.mkdir(parents=True, exist_ok=True)
    with (service.directory / "lock").open("w") as lock:
        fcntl.flock(lock, fcntl.LOCK_EX)
        service.owned()
        if args.action == "register":
            added = hotkey.register()
            try:
                service.unregister()
            except Exception:
                if added:
                    hotkey.unregister()
                raise
        else:
            service.unregister()
            hotkey.unregister()
    print(f"Quick terminal {args.action} complete")


if __name__ == "__main__":
    try:
        main()
    except (OSError, ValueError, RuntimeError, subprocess.CalledProcessError) as error:
        print(error, file=sys.stderr)
        sys.exit(1)
