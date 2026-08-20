#!/usr/bin/env python3
"""Package one Go executable for PyPI, npm, and GitHub Releases."""

import argparse
import base64
import csv
import hashlib
import io
import json
import platform
import re
import shutil
import stat
import subprocess
import tarfile
import zipfile
from pathlib import Path


PLATFORMS = {
    "linux-x64": (
        "linux",
        "x64",
        "tar.gz",
        ("manylinux_2_17_x86_64", "musllinux_1_2_x86_64"),
    ),
    "linux-arm64": (
        "linux",
        "arm64",
        "tar.gz",
        ("manylinux_2_17_aarch64", "musllinux_1_2_aarch64"),
    ),
    "darwin-x64": ("darwin", "x64", "tar.gz", ("macosx_12_0_x86_64",)),
    "darwin-arm64": ("darwin", "arm64", "tar.gz", ("macosx_12_0_arm64",)),
    "win32-x64": ("win32", "x64", "zip", ("win_amd64",)),
}
HOST_PLATFORMS = {
    ("linux", "x86_64"): "linux-x64",
    ("linux", "amd64"): "linux-x64",
    ("linux", "aarch64"): "linux-arm64",
    ("linux", "arm64"): "linux-arm64",
    ("darwin", "x86_64"): "darwin-x64",
    ("darwin", "arm64"): "darwin-arm64",
    ("windows", "amd64"): "win32-x64",
    ("windows", "x86_64"): "win32-x64",
}
MARKERS = (b"DataTables 3.0.2", b"--csvtotable-accent")


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--binary", type=Path, required=True)
    parser.add_argument("--platform", choices=PLATFORMS)
    parser.add_argument("--version", required=True)
    parser.add_argument("--dist", type=Path, default=Path("dist"))
    args = parser.parse_args()

    target = args.platform or HOST_PLATFORMS.get(
        (platform.system().lower(), platform.machine().lower())
    )
    if not target:
        raise SystemExit("unsupported host platform; pass --platform explicitly")
    version = args.version[1:] if args.version.startswith("v") else args.version
    wheel_version = python_version(version)
    repository = Path(__file__).resolve().parent.parent
    npm_package = json.loads((repository / "npm/package.json").read_text())
    versions = {
        "pyproject.toml": (
            project_version(repository / "pyproject.toml"),
            wheel_version,
        ),
        "npm/package.json": (npm_package["version"], version),
    }
    mismatches = [name for name, values in versions.items() if values[0] != values[1]]
    if mismatches:
        raise SystemExit(f"version does not match v{version}: {', '.join(mismatches)}")
    if any(value != version for value in npm_package["optionalDependencies"].values()):
        raise SystemExit(f"native npm dependency version does not match v{version}")

    binary = args.binary.read_bytes()
    missing = [marker.decode() for marker in MARKERS if marker not in binary]
    if missing:
        raise SystemExit(f"binary is missing embedded frontend markers: {missing}")

    npm_os, npm_cpu, archive_format, wheel_tags = PLATFORMS[target]
    executable_name = "csvtotable.exe" if npm_os == "win32" else "csvtotable"
    args.dist.mkdir(parents=True, exist_ok=True)
    for wheel_tag in wheel_tags:
        build_wheel(
            repository, args.dist, binary, executable_name, wheel_version, wheel_tag
        )

    stage = args.dist / f"npm-{target}"
    shutil.rmtree(stage, ignore_errors=True)
    bin_dir = stage / "bin"
    bin_dir.mkdir(parents=True)
    executable = bin_dir / executable_name
    executable.write_bytes(binary)
    executable.chmod(executable.stat().st_mode | stat.S_IXUSR | stat.S_IXGRP | stat.S_IXOTH)
    package = {
        "name": f"@vividvilla/csvtotable-{target}",
        "version": version,
        "description": f"csvtotable native binary for {target}",
        "license": "MIT",
        "repository": npm_package["repository"],
        "os": [npm_os],
        "cpu": [npm_cpu],
        "files": ["bin"],
    }
    (stage / "package.json").write_text(json.dumps(package, indent=2) + "\n")

    archive_stem = f"csvtotable-{version}-{target}"
    if archive_format == "zip":
        with zipfile.ZipFile(
            args.dist / f"{archive_stem}.zip", "w", zipfile.ZIP_DEFLATED
        ) as archive:
            archive.writestr(zip_entry(executable_name, 0o755), binary)
    else:
        with tarfile.open(args.dist / f"{archive_stem}.tar.gz", "w:gz") as archive:
            archive.add(executable, arcname=executable_name)

    npm = shutil.which("npm")
    if not npm:
        raise SystemExit("npm is required to create the native package")
    subprocess.run(
        [
            npm,
            "pack",
            str(stage.resolve()),
            "--pack-destination",
            str(args.dist.resolve()),
        ],
        check=True,
    )


def build_wheel(
    repository: Path,
    dist: Path,
    binary: bytes,
    executable_name: str,
    version: str,
    platform_tag: str,
) -> None:
    distribution = f"csvtotable-{version}"
    info = f"{distribution}.dist-info"
    script = f"{distribution}.data/scripts/{executable_name}"
    metadata = f"""Metadata-Version: 2.1
Name: csvtotable
Version: {version}
Summary: Convert CSV files into searchable and sortable HTML tables
License: MIT
Requires-Python: >=3.8
Project-URL: Homepage, https://github.com/vividvilla/csvtotable
Project-URL: Issues, https://github.com/vividvilla/csvtotable/issues
Description-Content-Type: text/markdown

""".encode() + (repository / "README.md").read_bytes()
    wheel = f"""Wheel-Version: 1.0
Generator: csvtotable
Root-Is-Purelib: false
Tag: py3-none-{platform_tag}
""".encode()
    entries = [
        (script, binary, 0o755),
        (f"{info}/METADATA", metadata, 0o644),
        (f"{info}/WHEEL", wheel, 0o644),
        (f"{info}/licenses/LICENSE", (repository / "LICENSE").read_bytes(), 0o644),
    ]
    record_rows = [
        [name, f"sha256={digest(data)}", str(len(data))] for name, data, _ in entries
    ]
    record_rows.append([f"{info}/RECORD", "", ""])
    record_output = io.StringIO()
    csv.writer(record_output, lineterminator="\n").writerows(record_rows)
    entries.append((f"{info}/RECORD", record_output.getvalue().encode(), 0o644))

    filename = dist / f"{distribution}-py3-none-{platform_tag}.whl"
    with zipfile.ZipFile(filename, "w", zipfile.ZIP_DEFLATED) as archive:
        for name, data, mode in entries:
            archive.writestr(zip_entry(name, mode), data)


def digest(data: bytes) -> str:
    return base64.urlsafe_b64encode(hashlib.sha256(data).digest()).rstrip(b"=").decode()


def python_version(version: str) -> str:
    match = re.fullmatch(
        r"(\d+\.\d+\.\d+)(?:-(alpha|beta|rc)\.(\d+))?", version
    )
    if not match:
        raise SystemExit("version must be X.Y.Z or X.Y.Z-(alpha|beta|rc).N")
    base, prerelease, number = match.groups()
    if not prerelease:
        return base
    suffix = {"alpha": "a", "beta": "b", "rc": "rc"}[prerelease]
    return f"{base}{suffix}{number}"


def project_version(path: Path) -> str:
    match = re.search(
        r'^version\s*=\s*"([^"]+)"\s*$', path.read_text(), re.MULTILINE
    )
    if not match:
        raise SystemExit(f"version not found in {path}")
    return match.group(1)


def zip_entry(name: str, mode: int) -> zipfile.ZipInfo:
    entry = zipfile.ZipInfo(name, (1980, 1, 1, 0, 0, 0))
    entry.compress_type = zipfile.ZIP_DEFLATED
    entry.create_system = 3
    entry.external_attr = (stat.S_IFREG | mode) << 16
    return entry


if __name__ == "__main__":
    main()
