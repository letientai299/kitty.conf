import json
import os
import shutil
import signal
import stat
import subprocess
import sys
import tempfile
import time
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]


class IntegrationTests(unittest.TestCase):
    def test_launcher_start_error(self):
        with tempfile.TemporaryDirectory() as directory:
            fake = Path(directory)
            executable = fake / "kitty"
            executable.write_text("#!/missing/interpreter\n")
            executable.chmod(0o755)
            (fake / "kitten").touch()
            result = subprocess.run(
                [str(ROOT / "bin/kt")],
                env=dict(
                    os.environ,
                    MISE_SHIMS_DIR=str(fake),
                    PATH=f"{fake}:{os.environ['PATH']}",
                ),
                capture_output=True,
                text=True,
                timeout=5,
            )
            self.assertNotEqual(result.returncode, 0)
            self.assertIn("kitty launch failed:", result.stderr)

    def test_launcher(self):
        with tempfile.TemporaryDirectory(prefix="kitty test ") as directory:
            fake = Path(directory)
            record = fake / "record.json"
            executable = fake / "kitty"
            executable.write_text(
                f"#!{sys.executable}\n"
                "import json, os, sys\n"
                "from pathlib import Path\n"
                "state = os.environ['KITTY_REPO_STATE']\n"
                "Path(os.environ['TEST_RECORD']).write_text(json.dumps({\n"
                "'args': sys.argv[1:], 'env': dict(os.environ),\n"
                "'mode': os.stat(state).st_mode & 0o777}))\n"
            )
            executable.chmod(0o755)
            (fake / "kitten").write_text("unused")
            env = dict(
                os.environ,
                MISE_SHIMS_DIR=str(fake),
                PATH=f"{fake}:{os.environ['PATH']}",
                KITTY_CONFIG_DIRECTORY="/old/config",
                KITTY_LISTEN_ON="unix:/old/socket",
                KITTY_WINDOW_ID="123",
                TEST_RECORD=str(record),
            )
            installed = fake / "kt"
            installed.symlink_to(ROOT / "bin/kt")
            states = []
            for args in [[], ["--", "printf", "argument with spaces"]]:
                subprocess.run(
                    [str(installed), *args],
                    cwd=fake,
                    env=env,
                    check=True,
                )
                deadline = time.monotonic() + 5
                while not record.exists() and time.monotonic() < deadline:
                    time.sleep(0.01)
                result = json.loads(record.read_text())
                record.unlink()
                self.assertEqual(
                    result["env"]["KITTY_CONFIG_DIRECTORY"], str(ROOT / "conf")
                )
                self.assertNotIn("KITTY_LISTEN_ON", result["env"])
                self.assertNotIn("KITTY_WINDOW_ID", result["env"])
                self.assertEqual(result["mode"], stat.S_IRWXU)
                self.assertEqual(
                    result["args"][result["args"].index("--") :], args or ["--"]
                )
                self.assertNotIn("--single-instance", result["args"])
                state = result["env"]["KITTY_REPO_STATE"]
                states.append(state)
                deadline = time.monotonic() + 5
                while Path(state).exists() and time.monotonic() < deadline:
                    time.sleep(0.01)
                self.assertFalse(Path(state).exists())
            self.assertNotEqual(*states)

    def test_launcher_signal(self):
        with tempfile.TemporaryDirectory() as directory:
            fake = Path(directory)
            record = fake / "state"
            executable = fake / "kitty"
            executable.write_text(
                f"#!{sys.executable}\n"
                "import json, os, signal, sys, time\n"
                "from pathlib import Path\n"
                "signal.signal(signal.SIGTERM, lambda *_: sys.exit(0))\n"
                "Path(os.environ['TEST_RECORD']).write_text(json.dumps({"
                "'state': os.environ['KITTY_REPO_STATE'], 'pid': os.getpid()}))\n"
                "time.sleep(15)\n"
            )
            executable.chmod(0o755)
            (fake / "kitten").touch()
            started = time.monotonic()
            subprocess.run(
                [str(ROOT / "bin/kt")],
                env=dict(
                    os.environ,
                    MISE_SHIMS_DIR=str(fake),
                    PATH=f"{fake}:{os.environ['PATH']}",
                    TEST_RECORD=str(record),
                ),
                check=True,
                capture_output=True,
                timeout=5,
            )
            self.assertLess(time.monotonic() - started, 2)
            state = None
            result = None
            try:
                deadline = time.monotonic() + 5
                while not record.exists() and time.monotonic() < deadline:
                    time.sleep(0.01)
                self.assertTrue(record.exists())
                result = json.loads(record.read_text())
                state = Path(result["state"])
                self.assertTrue(state.exists())
                os.kill(result["pid"], signal.SIGTERM)
                deadline = time.monotonic() + 5
                while state.exists() and time.monotonic() < deadline:
                    time.sleep(0.01)
                self.assertFalse(state.exists())
            finally:
                if result is not None and state is not None and state.exists():
                    os.kill(result["pid"], signal.SIGTERM)

    @unittest.skipUnless(shutil.which("kitty"), "Kitty is unavailable")
    def test_kitty_runtime(self):
        with tempfile.TemporaryDirectory() as directory:
            subprocess.run(
                [
                    "kitty",
                    "+runpy",
                    "import runpy; runpy.run_path('scripts/check-kitty.py', run_name='__main__')",
                ],
                cwd=ROOT,
                env=dict(
                    os.environ,
                    KITTY_CONFIG_DIRECTORY=str(ROOT / "conf"),
                    KITTY_REPO_ROOT=str(ROOT),
                    KITTY_REPO_STATE=directory,
                ),
                check=True,
            )
