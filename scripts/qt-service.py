import argparse
import ctypes
import fcntl
import json
import os
import plistlib
import shlex
import shutil
import subprocess
import sys
import tempfile
from pathlib import Path

NAME = "Kitty Config Quick Terminal"
BUNDLE = "local.kitty-conf.quick-terminal"
SERVICE = f"{BUNDLE} - {NAME} - runWorkflowAsService"
FIELDS = {"key_equivalent": "@\x1b", "enabled_services_menu": True}
PBS = "/System/Library/CoreServices/pbs"


class Preferences:
    def __init__(self):
        self.cf = ctypes.CDLL(
            "/System/Library/Frameworks/CoreFoundation.framework/CoreFoundation"
        )
        pointer = ctypes.c_void_p
        signatures = {
            "CFStringCreateWithCString": (
                [pointer, ctypes.c_char_p, ctypes.c_uint],
                pointer,
            ),
            "CFPreferencesCopyAppValue": ([pointer, pointer], pointer),
            "CFPreferencesSetAppValue": ([pointer, pointer, pointer], None),
            "CFPreferencesAppSynchronize": ([pointer], ctypes.c_bool),
            "CFPropertyListCreateData": (
                [pointer, pointer, ctypes.c_long, ctypes.c_ulong, pointer],
                pointer,
            ),
            "CFPropertyListCreateWithData": (
                [pointer, pointer, ctypes.c_ulong, pointer, pointer],
                pointer,
            ),
            "CFDataCreate": ([pointer, ctypes.c_char_p, ctypes.c_long], pointer),
            "CFDataGetLength": ([pointer], ctypes.c_long),
            "CFDataGetBytePtr": ([pointer], pointer),
            "CFRelease": ([pointer], None),
        }
        for name, (args, result) in signatures.items():
            function = getattr(self.cf, name)
            function.argtypes, function.restype = args, result
        self.domain = self.string("pbs")
        self.key = self.string("NSServicesStatus")

    def string(self, text):
        return self.cf.CFStringCreateWithCString(None, text.encode(), 0x08000100)

    def read(self):
        self.cf.CFPreferencesAppSynchronize(self.domain)
        value = self.cf.CFPreferencesCopyAppValue(self.key, self.domain)
        if not value:
            return {}
        try:
            data = self.cf.CFPropertyListCreateData(None, value, 200, 0, None)
            if not data:
                raise RuntimeError("Services preferences serialization failed")
            try:
                result = plistlib.loads(
                    ctypes.string_at(
                        self.cf.CFDataGetBytePtr(data), self.cf.CFDataGetLength(data)
                    )
                )
                if not isinstance(result, dict):
                    raise RuntimeError("Services preferences are invalid")
                return result
            finally:
                self.cf.CFRelease(data)
        finally:
            self.cf.CFRelease(value)

    def write(self, values):
        encoded = plistlib.dumps(values, fmt=plistlib.FMT_BINARY)
        data = self.cf.CFDataCreate(None, encoded, len(encoded))
        value = self.cf.CFPropertyListCreateWithData(None, data, 0, None, None)
        try:
            if not value:
                raise RuntimeError("Services preferences parsing failed")
            self.cf.CFPreferencesSetAppValue(self.key, value, self.domain)
            if not self.cf.CFPreferencesAppSynchronize(self.domain):
                raise RuntimeError("Services preferences synchronization failed")
        finally:
            if value:
                self.cf.CFRelease(value)
            self.cf.CFRelease(data)


def atomic_write(path, content):
    path.parent.mkdir(parents=True, exist_ok=True)
    fd, temporary = tempfile.mkstemp(dir=path.parent)
    try:
        with os.fdopen(fd, "wb") as stream:
            stream.write(content)
        os.replace(temporary, path)
    finally:
        Path(temporary).unlink(missing_ok=True)


def workflow_files(root):
    command = f"exec {shlex.quote(str(root / 'bin/kt'))} qt"
    info = {
        "CFBundleIdentifier": BUNDLE,
        "NSServices": [
            {
                "NSMenuItem": {"default": NAME},
                "NSMessage": "runWorkflowAsService",
                "NSRequiredContext": {},
                "NSSendTypes": [],
                "NSReturnTypes": [],
            }
        ],
    }
    workflow = {
        "AMDocumentVersion": "2",
        "actions": [
            {
                "action": {
                    "ActionBundlePath": "/System/Library/Automator/Run Shell Script.action",
                    "ActionName": "Run Shell Script",
                    "ActionParameters": {
                        "COMMAND_STRING": command,
                        "shell": "/bin/sh",
                        "inputMethod": 0,
                        "source": "",
                    },
                    "AMAccepts": {
                        "Container": "List",
                        "Optional": True,
                        "Types": ["com.apple.cocoa.string"],
                    },
                    "AMProvides": {
                        "Container": "List",
                        "Types": ["com.apple.cocoa.string"],
                    },
                    "AMActionVersion": "2.0.3",
                    "BundleIdentifier": "com.apple.RunShellScript",
                    "Class Name": "RunShellScriptAction",
                    "UUID": "C87CF92F-4EBB-4AA0-82E8-C9DF2C96853D",
                }
            }
        ],
        "workflowMetaData": {
            "serviceInputTypeIdentifier": "com.apple.Automator.nothing",
            "serviceOutputTypeIdentifier": "com.apple.Automator.nothing",
            "serviceProcessesInput": 0,
            "workflowTypeIdentifier": "com.apple.Automator.servicesMenu",
        },
    }
    return {
        "Info.plist": plistlib.dumps(info),
        "document.wflow": plistlib.dumps(workflow),
    }


def refresh():
    result = subprocess.run(
        [PBS, "-update"], capture_output=True, text=True, check=True
    )
    if "error" in (result.stdout + result.stderr).lower():
        raise RuntimeError("macOS Services refresh failed")


def check_service(workflow):
    result = subprocess.run([PBS, "-dump"], capture_output=True, text=True, check=True)
    if str(workflow) not in result.stdout or BUNDLE not in result.stdout:
        raise RuntimeError("macOS Service discovery failed")


class Registration:
    def __init__(self, root, home, preferences, update=refresh, discover=check_service):
        self.root = root.resolve()
        self.workflow = home / "Library/Services" / f"{NAME}.workflow"
        self.directory = home / "Library/Application Support/kitty-conf/quick-terminal"
        self.metadata = self.directory / "registration.json"
        self.preferences, self.update, self.discover = preferences, update, discover

    def owned(self):
        if not self.metadata.exists():
            if self.workflow.exists() or self.workflow.is_symlink():
                raise RuntimeError("Quick terminal workflow is not owned")
            return None
        state = json.loads(self.metadata.read_text())
        if state.get("version") != 1 or state.get("root") != str(self.root):
            raise RuntimeError(
                "Quick terminal registration belongs to another checkout"
            )
        if self.workflow.is_symlink():
            raise RuntimeError("Quick terminal workflow is a symlink")
        if self.workflow.exists():
            if any(path.is_symlink() for path in self.workflow.rglob("*")):
                raise RuntimeError("Quick terminal workflow contains a symlink")
            files = state["files"]
            actual = {
                str(p.relative_to(self.workflow))
                for p in self.workflow.rglob("*")
                if not p.is_dir()
            }
            if not actual <= {f"Contents/{name}" for name in files}:
                raise RuntimeError("Quick terminal workflow changed after registration")
            for name, content in files.items():
                path = self.workflow / "Contents" / name
                if path.is_symlink() or (path.exists() and path.read_text() != content):
                    raise RuntimeError(
                        "Quick terminal workflow changed after registration"
                    )
        return state

    def save(self, state):
        atomic_write(self.metadata, json.dumps(state).encode())

    def restore(self, state):
        values = self.preferences.read()
        entry = values.get(SERVICE, {}).copy()
        for key, installed in FIELDS.items():
            if entry.get(key) == installed:
                if key in state["previous"]:
                    entry[key] = state["previous"][key]
                else:
                    entry.pop(key, None)
        if entry:
            values[SERVICE] = entry
        else:
            values.pop(SERVICE, None)
        self.preferences.write(values)

    def register(self):
        old = self.owned()
        values = self.preferences.read()
        for key, entry in values.items():
            if (
                key != SERVICE
                and isinstance(entry, dict)
                and entry.get("key_equivalent") == FIELDS["key_equivalent"]
            ):
                raise RuntimeError("Cmd+Esc belongs to another Service")
        current = values.get(SERVICE, {})
        if not isinstance(current, dict):
            raise RuntimeError("Quick terminal preferences are invalid")
        if old and any(current.get(key) != value for key, value in FIELDS.items()):
            raise RuntimeError("Quick terminal shortcut changed after registration")
        files = workflow_files(self.root)
        state = old or {
            "version": 1,
            "root": str(self.root),
            "previous": {key: current[key] for key in FIELDS if key in current},
        }
        state = dict(
            state, files={name: content.decode() for name, content in files.items()}
        )
        # Persist recovery before external mutations.
        self.save(state)
        try:
            for name, content in files.items():
                atomic_write(self.workflow / "Contents" / name, content)
            self.update()
            self.discover(self.workflow)
            latest = self.preferences.read()
            latest[SERVICE] = dict(latest.get(SERVICE, {}), **FIELDS)
            self.preferences.write(latest)
            self.update()
            actual = self.preferences.read().get(SERVICE, {})
            if any(actual.get(key) != value for key, value in FIELDS.items()):
                raise RuntimeError("Quick terminal shortcut verification failed")
        except Exception:
            if old:
                for name, content in old["files"].items():
                    atomic_write(self.workflow / "Contents" / name, content.encode())
                self.save(old)
            else:
                self.restore(state)
                if self.workflow.exists():
                    shutil.rmtree(self.workflow)
                self.update()
                self.metadata.unlink()
            raise

    def unregister(self):
        state = self.owned()
        if state is None:
            return
        self.restore(state)
        if self.workflow.exists():
            shutil.rmtree(self.workflow)
        self.update()
        self.metadata.unlink()


def main():
    parser = argparse.ArgumentParser(description="Manage the quick terminal Service")
    parser.add_argument("action", choices=("register", "unregister"))
    args = parser.parse_args()
    if sys.platform != "darwin":
        raise RuntimeError("Quick terminal registration requires macOS")
    root = Path(__file__).resolve().parents[1]
    if args.action == "register" and not os.access(root / "bin/kt", os.X_OK):
        raise RuntimeError("Quick terminal executable is missing")
    registration = Registration(root, Path.home(), Preferences())
    registration.directory.mkdir(parents=True, exist_ok=True)
    with (registration.directory / "lock").open("w") as lock:
        fcntl.flock(lock, fcntl.LOCK_EX)
        getattr(registration, args.action)()
    print(f"Quick terminal {args.action} complete")


if __name__ == "__main__":
    try:
        main()
    except (OSError, ValueError, RuntimeError, subprocess.CalledProcessError) as error:
        print(error, file=sys.stderr)
        sys.exit(1)
