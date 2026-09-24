import atexit
import os
import shutil
import sys
import tempfile
from pathlib import Path

from kitty.constants import kitten_exe

root = Path(__file__).resolve().parents[1]
pid = str(os.getpid())
state = os.environ.get("KITTY_REPO_STATE")
if not state or (
    os.environ.get("KITTY_PID") and os.environ.get("KITTY_REPO_OWNER") != pid
):
    state = tempfile.mkdtemp(prefix="kitty-global-", dir="/tmp")
    atexit.register(shutil.rmtree, state, ignore_errors=True)

environment = {
    "KITTY_CONFIG_DIRECTORY": str(root / "conf"),
    "KITTY_REPO_ROOT": str(root),
    "KITTY_REPO_STATE": state,
    "KITTY_REPO_KITTEN": kitten_exe(),
    "KITTY_REPO_OWNER": pid,
}
os.environ.update(environment)
for name, value in environment.items():
    print(f"env {name}={value}")
print(f"listen_on unix:{state}/rc")
print(
    f"scrollback_pager {root}/bin/kt pager INPUT_LINE_NUMBER CURSOR_LINE CURSOR_COLUMN"
)
if sys.platform == "darwin":
    print(f"include {root}/conf/mac.conf")
