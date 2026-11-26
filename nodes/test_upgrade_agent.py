"""Exercise upgrades of a synthetic DynamicUser service on the CI runner."""

import hashlib
import json
import os
import secrets
import shutil
import stat
import subprocess
import sys
import tempfile
import time
from pathlib import Path


def systemctl(*args):
    return subprocess.check_output(["systemctl", *args], text=True, timeout=20).strip()


def wait_for_version(unit, expected):
    until = time.monotonic() + 5
    while time.monotonic() < until:
        pid = int(systemctl("show", "--property=MainPID", "--value", unit))
        if pid:
            try:
                args = Path(f"/proc/{pid}/cmdline").read_bytes().split(b"\0")
            except FileNotFoundError:
                args = []
            if args[1:2] == [expected]:
                return
        time.sleep(0.05)
    raise AssertionError(f"{unit} did not run the expected executable")


def run_upgrade_fixture():
    node_id = os.getpid()
    owner = secrets.token_hex(16)
    slot = f"{node_id}-{owner[:12]}"
    unit = f"s-ui-node-{slot}.service"
    unit_path = Path("/etc/systemd/system") / unit
    base = Path("/opt/s-ui-nodes")
    base.mkdir(mode=0o755, exist_ok=True)
    folder = base / slot
    folder.mkdir(mode=0o755)
    created_unit = False
    try:
        binary = folder / "agent"
        executable_mode = stat.S_IMODE(Path("/bin/sleep").stat().st_mode)
        with os.fdopen(os.open(binary, os.O_CREAT | os.O_EXCL | os.O_WRONLY, executable_mode), "wb") as f:
            f.write(b"#!/bin/sh\nexec /bin/sleep 301\n")
        candidate = b"#!/bin/sh\nexec /bin/sleep 302\n"
        staged = folder / "agent.next"
        with os.fdopen(os.open(staged, os.O_CREAT | os.O_EXCL | os.O_WRONLY, 0o600), "wb") as f:
            f.write(candidate)
        with unit_path.open("x") as f:
            created_unit = True
            f.write(f"# S-UI managed node {node_id} owner={owner}\n"
                    f"[Service]\nDynamicUser=yes\nExecStart={binary}\n")
        systemctl("daemon-reload")
        systemctl("start", unit)
        wait_for_version(unit, b"301")

        # Exercise the real unit restart and explicit rollback without
        # leaving a delayed watchdog scheduled on the runner.
        with tempfile.TemporaryDirectory() as temp:
            (Path(temp) / "systemd-run").symlink_to("/usr/bin/true")
            env = {**os.environ, "PATH": temp + os.pathsep + os.environ["PATH"]}
            script = Path(__file__).with_name("upgrade_agent.py")
            for action, expected in [("activate", b"302"), ("rollback", b"301")]:
                request = {"id": node_id, "owner": owner, "action": action,
                           "sha256": hashlib.sha256(candidate).hexdigest()}
                subprocess.run([sys.executable, str(script)], input=json.dumps(request),
                               text=True, env=env, check=True, timeout=45)
                wait_for_version(unit, expected)
                assert stat.S_IMODE(binary.stat().st_mode) == executable_mode
    finally:
        if created_unit:
            systemctl("stop", unit)
            unit_path.unlink()
            systemctl("daemon-reload")
        shutil.rmtree(folder)


if __name__ == "__main__":
    run_upgrade_fixture()
