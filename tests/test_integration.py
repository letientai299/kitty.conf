import json
import os
import shutil
import stat
import subprocess
import sys
import tempfile
import time
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]


class IntegrationTests(unittest.TestCase):
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
                result = json.loads(record.read_text())
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
                self.assertFalse(Path(state).exists())
            self.assertNotEqual(*states)

    def test_launcher_signal(self):
        with tempfile.TemporaryDirectory() as directory:
            fake = Path(directory)
            record = fake / "state"
            executable = fake / "kitty"
            executable.write_text(
                f"#!{sys.executable}\n"
                "import os, signal, sys, time\n"
                "from pathlib import Path\n"
                "signal.signal(signal.SIGTERM, lambda *_: sys.exit(0))\n"
                "Path(os.environ['TEST_RECORD']).write_text(os.environ['KITTY_REPO_STATE'])\n"
                "time.sleep(15)\n"
            )
            executable.chmod(0o755)
            (fake / "kitten").touch()
            proc = subprocess.Popen(
                [str(ROOT / "bin/kt")],
                env=dict(
                    os.environ,
                    PATH=f"{fake}:{os.environ['PATH']}",
                    TEST_RECORD=str(record),
                ),
            )
            try:
                deadline = time.monotonic() + 5
                while not record.exists() and time.monotonic() < deadline:
                    time.sleep(0.01)
                self.assertTrue(record.exists())
                state = Path(record.read_text())
                proc.terminate()
                self.assertEqual(proc.wait(timeout=5), 0)
                self.assertFalse(state.exists())
            finally:
                if proc.poll() is None:
                    proc.terminate()
                    proc.wait(timeout=20)

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
