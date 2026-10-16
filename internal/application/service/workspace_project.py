"""Project discovery helpers executed inside the session sandbox."""
import concurrent.futures
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

RESERVED_PORTS = {49983, 5900, 6080, 9222}
SKIP_DIRS = {"node_modules", "input", "output", "vendor", "venv", "__pycache__"}


def listening_ports():
    ports = set()
    for table in ("/proc/net/tcp", "/proc/net/tcp6"):
        try:
            for line in Path(table).read_text().splitlines()[1:]:
                fields = line.split()
                if fields[3] == "0A":
                    port = int(fields[1].split(":")[-1], 16)
                    if port >= 1024 and port not in RESERVED_PORTS:
                        ports.add(port)
        except OSError:
            pass
    preferred = [5173, 3000, 4173, 8080, 8000, 3001]
    return sorted(ports, key=lambda p: (preferred.index(p) if p in preferred else 99, p))[:32]


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


def running_url(preferred=""):
    if preferred and serves_html(preferred):
        return preferred
    urls = ["http://127.0.0.1:%d/" % port for port in listening_ports()]
    with concurrent.futures.ThreadPoolExecutor(max_workers=8) as pool:
        results = list(pool.map(serves_html, urls))
    return next((url for url, ready in zip(urls, results) if ready), "")


def project_command(root):
    """Bounded discovery; prefer frontend packages over a sibling API server."""
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
                script = next((key for key in ("dev", "start", "serve") if isinstance(scripts.get(key), str)), None)
                if script:
                    manager = "pnpm" if (directory / "pnpm-lock.yaml").exists() else "yarn" if (directory / "yarn.lock").exists() else "npm"
                    if not shutil.which(manager):
                        manager = "npm"
                    candidates.append((score, str(directory), [manager, "run", script]))
                    continue
                continue
            except (OSError, ValueError, TypeError, AttributeError):
                continue
        if "index.html" in files and not (directory / "index.html").is_symlink():
            candidates.append((score + 20, str(directory), None))
    if not candidates:
        return None
    _, directory, command = sorted(candidates, key=lambda c: (c[0], c[1]))[0]
    if command is None:
        with socket.socket() as sock:
            sock.bind(("127.0.0.1", 0))
            port = sock.getsockname()[1]
        command = [sys.executable, "-m", "http.server", str(port), "--bind", "127.0.0.1"]
    return directory, command


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
