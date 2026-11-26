import fcntl
import hashlib
import json
import os
import platform
import re
import secrets
import stat
import subprocess
import sys
import time
from pathlib import Path


# Runs under systemd independently of the SSH session. Keep all rollback
# information with the executable so inspection can also recover after reboot.
rollback_code = """
import fcntl, json, os, subprocess, sys
from pathlib import Path

def sync_folder(folder):
    fd = os.open(folder, os.O_RDONLY | os.O_DIRECTORY)
    try:
        os.fsync(fd)
    finally:
        os.close(fd)

def rollback(folder, service, marker, backup):
    if backup.is_file():
        os.replace(backup, folder / 'agent')
        sync_folder(folder)
    subprocess.run(['systemctl', 'restart', service], check=True, timeout=40)
    marker.unlink()
    sync_folder(folder)

if __name__ == '__main__' and len(sys.argv) == 4:
    folder, service, trial = Path(sys.argv[1]), sys.argv[2], sys.argv[3]
    marker, backup = folder / 'upgrade.pending', folder / 'agent.previous'
    with os.fdopen(os.open(folder / 'upgrade.lock', os.O_CREAT | os.O_NOFOLLOW | os.O_RDWR, 0o600), 'r+') as lock:
        fcntl.flock(lock, fcntl.LOCK_EX)
        if marker.is_file() and json.loads(marker.read_text())['trial'] == trial:
            rollback(folder, service, marker, backup)
"""

helpers = {"__name__": "_helpers"}
exec(rollback_code, helpers)
sync_folder, rollback = helpers["sync_folder"], helpers["rollback"]


def require_root_owned(metadata, kind, role):
    if metadata.st_uid != 0 or not kind(metadata.st_mode) or metadata.st_mode & (stat.S_IWGRP | stat.S_IWOTH):
        raise SystemExit(f"unsafe {role} ownership or permissions")
    return metadata


def prepare_candidate(staged, binary, digest):
    installed = require_root_owned(binary.lstat(), stat.S_ISREG, "installed binary")
    mode = stat.S_IMODE(installed.st_mode)
    if not mode & stat.S_IXUSR or mode & (stat.S_ISUID | stat.S_ISGID | stat.S_ISVTX):
        raise SystemExit("unsafe installed executable mode")

    # Apply the installed binary's access policy to the verified inode. In
    # particular, a DynamicUser service needs the existing execute permission.
    with os.fdopen(os.open(staged, os.O_RDWR | os.O_NOFOLLOW | os.O_NONBLOCK), "rb") as candidate:
        metadata = require_root_owned(os.fstat(candidate.fileno()), stat.S_ISREG, "staged binary")
        if metadata.st_nlink != 1:
            raise SystemExit("staged executable is shared")
        staged_hash = hashlib.sha256()
        for chunk in iter(lambda: candidate.read(1 << 20), b""):
            staged_hash.update(chunk)
        if staged_hash.hexdigest() != digest:
            raise SystemExit("agent checksum mismatch")
        if metadata.st_gid != installed.st_gid:
            os.fchown(candidate.fileno(), installed.st_uid, installed.st_gid)
        os.fchmod(candidate.fileno(), mode)
        os.fsync(candidate.fileno())


req = json.load(sys.stdin)
node_id = int(req["id"])
owner = req["owner"]
if os.geteuid() != 0 or node_id <= 0 or not re.fullmatch(r"[a-f0-9]{32}", owner):
    raise SystemExit("invalid installation")
slot = f"{node_id}-{owner[:12]}"
service = f"s-ui-node-{slot}.service"
unit = Path("/etc/systemd/system") / service
folder = Path("/opt/s-ui-nodes") / slot
if unit.is_symlink() or not unit.is_file() or f"# S-UI managed node {node_id} owner={owner}" not in unit.read_text().splitlines():
    raise SystemExit("foreign service")
require_root_owned(folder.parent.stat(), stat.S_ISDIR, "agent root directory")
require_root_owned(folder.lstat(), stat.S_ISDIR, "agent directory")

marker, backup = folder / "upgrade.pending", folder / "agent.previous"
binary, staged = folder / "agent", folder / "agent.next"
if any(path.is_symlink() for path in [marker, backup, binary, staged]):
    raise SystemExit("unsafe upgrade path")

action = req["action"]
digest = req["sha256"]
if action != "inspect" and not re.fullmatch(r"[a-f0-9]{64}", digest):
    raise SystemExit("invalid digest")

with os.fdopen(os.open(folder / "upgrade.lock", os.O_CREAT | os.O_NOFOLLOW | os.O_RDWR, 0o600), "r+") as lock:
    fcntl.flock(lock, fcntl.LOCK_EX)
    pending = json.loads(marker.read_text()) if marker.is_file() else None
    if action == "inspect":
        if pending and time.time() > pending["deadline"]:
            rollback(folder, service, marker, backup)
            pending = None
        arch = {"x86_64": "amd64", "aarch64": "arm64"}.get(platform.machine(), "")
        print(json.dumps({"arch": arch, "pending": bool(pending)}))
    elif action == "activate":
        if pending or not binary.is_file() or not staged.is_file():
            raise SystemExit("upgrade not ready")
        if subprocess.run(["systemctl", "is-active", "--quiet", service], timeout=10).returncode:
            raise SystemExit("agent is inactive")
        prepare_candidate(staged, binary, digest)
        backup.unlink(missing_ok=True)
        os.link(binary, backup)
        trial = secrets.token_hex(16)
        # Arm the watchdog before modifying the binary or recording the trial.
        subprocess.run([
            "systemd-run", "--quiet", "--collect", "--on-active=100s",
            "--timer-property=AccuracySec=1s",
            sys.executable, "-c", rollback_code, str(folder), service, trial,
        ], check=True, timeout=10)
        temporary = folder / ("upgrade-" + trial)
        with temporary.open("x") as state:
            json.dump({"digest": digest, "trial": trial, "deadline": time.time() + 100}, state)
            state.flush()
            os.fsync(state.fileno())
        os.replace(temporary, marker)
        sync_folder(folder)
        os.replace(staged, binary)
        sync_folder(folder)
        subprocess.run(["systemctl", "restart", service], check=True, timeout=40)
    elif action == "confirm":
        if not pending or pending["digest"] != digest:
            raise SystemExit("upgrade not pending")
        marker.unlink()
        sync_folder(folder)
        backup.unlink(missing_ok=True)
    elif action == "rollback":
        if pending and pending["digest"] == digest:
            rollback(folder, service, marker, backup)
    else:
        raise SystemExit("invalid upgrade action")
