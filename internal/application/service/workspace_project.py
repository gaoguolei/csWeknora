"""Project discovery helpers executed inside the session sandbox."""
import fcntl
import hashlib
import http.client
import json
import os
from pathlib import Path
import shutil
import socket
import subprocess
import sys
import time
from urllib.parse import urlsplit

SKIP_DIRS = {"node_modules", "input", "output", "vendor", "venv", "__pycache__", "dist", "build", "out", "src", "public"}


def serves_html(url):
    parsed = urlsplit(url)
    connection = http.client.HTTPConnection(parsed.hostname, parsed.port, timeout=0.6)
    try:
        connection.request("GET", (parsed.path or "/") + ("?" + parsed.query if parsed.query else ""))
        response = connection.getresponse()
        sample = response.read(1024).lower()
        return response.status < 400 and ("text/html" in (response.getheader("Content-Type") or "") or b"<!doctype html" in sample or b"<html" in sample)
    except (OSError, http.client.HTTPException):
        return False
    finally:
        connection.close()


def project_command(root):
    """Find a production build; never fall back to a development server."""
    candidates = []
    seen = 0
    for folder, dirs, files in os.walk(root, followlinks=False):
        directory = Path(folder)
        depth = len(directory.relative_to(root).parts)
        dirs[:] = sorted(d for d in dirs if d not in SKIP_DIRS and not d.startswith(".") and not (directory / d).is_symlink()) if depth < 3 else []
        seen += 1
        if seen > 160:
            break
        score = 0 if directory.name in ("frontend", "web", "client", "app") else 10
        if "package.json" in files and not (directory / "package.json").is_symlink():
            try:
                with (directory / "package.json").open() as manifest:
                    package = json.loads(manifest.read(262144))
                scripts = package.get("scripts", {})
                if isinstance(scripts.get("build"), str) and scripts["build"].strip():
                    manager = "pnpm" if (directory / "pnpm-lock.yaml").exists() else "yarn" if (directory / "yarn.lock").exists() else "npm"
                    if not shutil.which(manager):
                        manager = "npm"
                    script = next((key for key in ("preview", "start") if isinstance(scripts.get(key), str) and scripts[key].strip()), None)
                    candidates.append((score, str(directory), {
                        "manager": manager,
                        "build": [manager, "run", "build"],
                        "serve": [manager, "run", script] if script else None,
                        "serve_script": scripts.get(script, ""),
                    }))
                    continue
                continue
            except (OSError, ValueError, TypeError, AttributeError):
                continue
        if "index.html" in files and not (directory / "index.html").is_symlink():
            candidates.append((score + 20, str(directory), {"static": True}))
    if not candidates:
        return None
    _, directory, plan = sorted(candidates, key=lambda c: (c[0], c[1]))[0]
    return directory, plan


def alive(pid):
    try:
        os.kill(int(pid), 0)
        return Path("/proc/%s/stat" % int(pid)).read_text().split(") ", 1)[1][0] != "Z"
    except (OSError, ValueError, IndexError, TypeError):
        return False


def log_tail(path, limit=4000):
    try:
        with path.open("rb") as log:
            log.seek(0, os.SEEK_END)
            log.seek(max(0, log.tell() - limit))
            return log.read(limit).decode("utf-8", errors="replace")
    except OSError:
        return ""
