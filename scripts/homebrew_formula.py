#!/usr/bin/env python3
"""Generate the Homebrew formula for a csvtotable release."""

import argparse
import hashlib
from pathlib import Path


def artifact(version: str, dist: Path, target: str) -> tuple[str, str]:
    name = f"csvtotable-{version}-{target}.tar.gz"
    checksum = hashlib.sha256((dist / name).read_bytes()).hexdigest()
    url = (
        "https://github.com/vividvilla/csvtotable/releases/download/"
        f"v{version}/{name}"
    )
    return url, checksum


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--version", required=True)
    parser.add_argument("--dist", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()

    version = args.version.removeprefix("v")
    darwin_arm_url, darwin_arm_sha = artifact(version, args.dist, "darwin-arm64")
    darwin_x64_url, darwin_x64_sha = artifact(version, args.dist, "darwin-x64")
    linux_arm_url, linux_arm_sha = artifact(version, args.dist, "linux-arm64")
    linux_x64_url, linux_x64_sha = artifact(version, args.dist, "linux-x64")

    formula = f'''class Csvtotable < Formula
  desc "Convert CSV files into interactive HTML tables"
  homepage "https://github.com/vividvilla/csvtotable"
  version "{version}"
  license "MIT"

  on_macos do
    on_arm do
      url "{darwin_arm_url}"
      sha256 "{darwin_arm_sha}"
    end
    on_intel do
      url "{darwin_x64_url}"
      sha256 "{darwin_x64_sha}"
    end
  end

  on_linux do
    on_arm do
      url "{linux_arm_url}"
      sha256 "{linux_arm_sha}"
    end
    on_intel do
      url "{linux_x64_url}"
      sha256 "{linux_x64_sha}"
    end
  end

  def install
    bin.install "csvtotable"
  end

  test do
    assert_match version.to_s, shell_output("#{{bin}}/csvtotable --version")
  end
end
'''
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(formula)


if __name__ == "__main__":
    main()
