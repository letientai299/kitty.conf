import importlib.util
import shutil
import tempfile
import unittest
from pathlib import Path
from unittest.mock import Mock

SOURCE = Path(__file__).resolve().parents[1]
SPEC = importlib.util.spec_from_file_location(
    "qt_hotkey", SOURCE / "scripts/qt-hotkey.py"
)
assert SPEC and SPEC.loader
qt = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(qt)


class HotkeyTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.home = Path(self.temp.name)
        self.root = self.home / "checkout ' with $(touch nope)"
        scripts = self.root / "scripts"
        scripts.mkdir(parents=True)
        shutil.copy2(SOURCE / "scripts/qt-service.py", scripts / "qt-service.py")
        self.init = self.home / ".hammerspoon/init.lua"
        self.init.parent.mkdir()
        self.original = "-- user config\nDOCKCTL_ROOT = 'elsewhere'\n"
        self.init.write_text(self.original)
        self.hotkey = qt.Hotkey(self.root, self.home, "hs")
        self.hotkey.loaded = Mock()
        self.hotkey.reload = Mock()

    def test_round_trip(self):
        self.assertTrue(self.hotkey.register())
        self.assertFalse(self.hotkey.register())
        self.assertEqual(
            self.init.read_text(), self.original + qt.block(self.root.resolve())
        )
        self.assertIn("$(touch nope)", self.init.read_text())
        self.hotkey.unregister()
        self.hotkey.unregister()
        self.assertEqual(self.init.read_text(), self.original)
        self.hotkey.reload.assert_any_call(True)
        self.hotkey.reload.assert_any_call(False)
        self.assertFalse((self.home / "nope").exists())

    def test_preserves_other_config(self):
        self.hotkey.register()
        self.init.write_text(self.init.read_text() + "-- later edit\n")
        self.hotkey.unregister()
        self.assertEqual(self.init.read_text(), self.original + "-- later edit\n")

    def test_rejects_changed_block(self):
        self.hotkey.register()
        self.init.write_text(
            self.init.read_text().replace("hotkey unavailable", "changed")
        )
        with self.assertRaisesRegex(RuntimeError, "changed after"):
            self.hotkey.unregister()

    def test_rolls_back_failed_reload(self):
        self.hotkey.reload.side_effect = [RuntimeError("reload failed"), None]
        with self.assertRaisesRegex(RuntimeError, "reload failed"):
            self.hotkey.register()
        self.assertEqual(self.init.read_text(), self.original)

    def test_missing_config_unregisters(self):
        self.init.unlink()
        self.hotkey.unregister()


if __name__ == "__main__":
    unittest.main()
