import copy
import importlib.util
import plistlib
import subprocess
import tempfile
import unittest
from pathlib import Path

SPEC = importlib.util.spec_from_file_location(
    "qt_service", Path(__file__).resolve().parents[1] / "scripts/qt-service.py"
)
assert SPEC and SPEC.loader
qt = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(qt)


class Preferences:
    def __init__(self, values):
        self.values = copy.deepcopy(values)

    def read(self):
        return copy.deepcopy(self.values)

    def write(self, values):
        self.values = copy.deepcopy(values)


class RegistrationTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.home = Path(self.temp.name)
        self.root = self.home / "checkout ' with $(touch nope)"
        self.before = {
            "unrelated": {"key_equivalent": "@x", "enabled_services_menu": False},
            qt.SERVICE: {"key_equivalent": "@y", "custom": "preserve"},
        }
        self.prefs = Preferences(self.before)
        self.registration = qt.Registration(
            self.root, self.home, self.prefs, lambda: None, lambda _: None
        )

    def test_round_trip(self):
        self.registration.register()
        self.registration.register()
        self.assertEqual(self.prefs.values[qt.SERVICE]["key_equivalent"], "@\x1b")
        self.registration.unregister()
        self.registration.unregister()
        self.assertEqual(self.prefs.values, self.before)
        self.assertFalse(self.registration.workflow.exists())

    def test_preserves_later_edits(self):
        self.registration.register()
        self.prefs.values[qt.SERVICE]["key_equivalent"] = "@z"
        self.prefs.values["new"] = {"enabled_services_menu": True}
        self.registration.unregister()
        self.assertEqual(self.prefs.values[qt.SERVICE]["key_equivalent"], "@z")
        self.assertIn("new", self.prefs.values)

    def test_conflict(self):
        self.prefs.values["other"] = dict(qt.FIELDS)
        with self.assertRaisesRegex(RuntimeError, "another Service"):
            self.registration.register()
        self.assertFalse(self.registration.metadata.exists())

    def test_foreign_workflow(self):
        self.registration.workflow.mkdir(parents=True)
        with self.assertRaisesRegex(RuntimeError, "not owned"):
            self.registration.register()

    def test_other_checkout(self):
        self.registration.register()
        other = qt.Registration(self.home / "other", self.home, self.prefs)
        with self.assertRaisesRegex(RuntimeError, "another checkout"):
            other.unregister()

    def test_changed_workflow(self):
        self.registration.register()
        (self.registration.workflow / "user-file").write_text("keep")
        with self.assertRaisesRegex(RuntimeError, "changed after"):
            self.registration.unregister()

    def test_discovery_rollback(self):
        def fail(_):
            raise RuntimeError("discovery failed")

        self.registration.discover = fail
        with self.assertRaisesRegex(RuntimeError, "discovery failed"):
            self.registration.register()
        self.assertEqual(self.prefs.values, self.before)
        self.assertFalse(self.registration.workflow.exists())
        self.assertFalse(self.registration.metadata.exists())

    def test_missing_workflow(self):
        self.registration.register()
        qt.shutil.rmtree(self.registration.workflow)
        self.registration.unregister()
        self.assertEqual(self.prefs.values, self.before)

    def test_workflow_execution(self):
        binary = self.root / "bin/kt"
        binary.parent.mkdir(parents=True)
        binary.write_text("#!/bin/sh\nprintf '%s' \"$1\"\n")
        binary.chmod(0o700)
        document = plistlib.loads(qt.workflow_files(self.root)["document.wflow"])
        action = document["actions"][0]["action"]["ActionParameters"]
        result = subprocess.run(
            [action["shell"], "-c", action["COMMAND_STRING"]],
            cwd=self.home,
            check=True,
            capture_output=True,
            text=True,
        )
        self.assertEqual(result.stdout, "qt")
        self.assertFalse((self.home / "nope").exists())


if __name__ == "__main__":
    unittest.main()
