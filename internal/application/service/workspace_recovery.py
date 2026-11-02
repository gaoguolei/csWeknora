"""Restore only bounded, regular source files into a newly created workspace."""
import os
import shutil
import stat
import sys
import tempfile
import zipfile


def restore(archive_path, root, missing_only=False):
    root = os.path.abspath(root)
    if os.path.realpath(root) != root:
        raise ValueError("workspace must not be a symlink")
    os.makedirs(root, exist_ok=True)
    with zipfile.ZipFile(archive_path) as archive:
        entries = archive.infolist()
        if len(entries) > 20000:
            raise ValueError("backup exceeds 20000 files")
        total = 0
        targets = set()
        # Validate the entire manifest before touching the workspace.
        for entry in entries:
            parts = entry.filename.split("/")
            mode = entry.external_attr >> 16
            if (not entry.filename or "\\" in entry.filename
                    or any(part in ("", ".", "..", ".git") for part in parts)
                    or entry.is_dir()
                    or stat.S_IFMT(mode) not in (0, stat.S_IFREG)):
                raise ValueError("invalid source backup entry")
            target = os.path.join(root, *parts)
            if target in targets or os.path.realpath(target) != target:
                raise ValueError("duplicate or symlinked source backup path")
            targets.add(target)
            total += entry.file_size
            if total > 200 * 1024 * 1024:
                raise ValueError("backup exceeds 200 MiB")
        for entry in entries:
            target = os.path.join(root, entry.filename)
            os.makedirs(os.path.dirname(target), exist_ok=True)
            with archive.open(entry) as source:
                publish(source, target, (entry.external_attr >> 16) & 0o777, missing_only)


def publish(source, target, mode, missing_only=True):
    fd, staged = tempfile.mkstemp(prefix='.weknora-recover-', dir=os.path.dirname(target))
    try:
        with os.fdopen(fd, 'wb') as output:
            shutil.copyfileobj(source, output, 1024 * 1024)
        os.chmod(staged, mode)
        if missing_only:
            try:
                os.link(staged, target)
            except FileExistsError:
                return
        else:
            os.replace(staged, target)
    finally:
        if os.path.exists(staged):
            os.unlink(staged)



def restore_artifact(source, root, relative):
    """Publish one missing artifact without following links or replacing edits."""
    root = os.path.abspath(root)
    parts = relative.split('/')
    if (os.path.realpath(root) != root or '\\' in relative
            or any(part in ('', '.', '..', '.git') for part in parts)):
        raise ValueError('invalid artifact recovery path')
    target = os.path.join(root, *parts)
    if os.path.realpath(target) != target:
        raise ValueError('artifact recovery path contains symlink')
    os.makedirs(os.path.dirname(target), exist_ok=True)
    with open(source, 'rb') as data:
        publish(data, target, 0o644)


if __name__ == "__main__":
    try:
        if sys.argv[1] == '--artifact':
            restore_artifact(sys.argv[2], sys.argv[3], sys.argv[4])
        elif sys.argv[1] == '--missing':
            restore(sys.argv[2], sys.argv[3], missing_only=True)
        else:
            restore(sys.argv[1], sys.argv[2])
    except Exception as error:
        sys.exit("workspace recovery failed: " + str(error))
