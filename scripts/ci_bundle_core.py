#!/usr/bin/env python3
"""Download matching official sing-box and pack full offline zip.

Env:
  PLATFORM   e.g. windows-amd64
  CORE_BIN   sing-box.exe or sing-box
  CLIENT_EXT .exe or empty
  UA         User-Agent
"""
from __future__ import annotations

import io
import json
import os
import shutil
import tarfile
import urllib.request
import zipfile
from pathlib import Path


def main() -> None:
    platform = os.environ["PLATFORM"]
    core_bin = os.environ["CORE_BIN"]
    client_ext = os.environ.get("CLIENT_EXT", "")
    ua = os.environ.get("UA", "Swell-Box-CI")

    rel = json.loads(Path("dist/rel.json").read_text(encoding="utf-8"))
    print("core tag=", rel.get("tag_name"), flush=True)

    def ok(name: str) -> bool:
        n = name.lower()
        if platform not in n:
            return False
        if "android" in n or "legacy" in n:
            return False
        return n.endswith(".zip") or n.endswith(".tar.gz") or n.endswith(".tgz")

    def score(name: str) -> int:
        n = name.lower()
        s = 0
        if (
            n.endswith(platform + ".zip")
            or n.endswith(platform + ".tar.gz")
            or n.endswith(platform + ".tgz")
        ):
            s += 10
        if n.endswith(".zip"):
            s += 1
        return s

    cands = [a for a in rel.get("assets", []) if ok(a["name"])]
    cands.sort(key=lambda a: score(a["name"]), reverse=True)
    if not cands:
        raise SystemExit(f"no asset for {platform}")
    asset = cands[0]
    print("asset=", asset["name"], flush=True)

    headers = {"User-Agent": ua, "Accept": "application/octet-stream"}
    # Prefer token for API-style URLs / rate limits; browser_download_url is public but
    # authenticated requests are more reliable in parallel CI matrix jobs.
    token = os.environ.get("GITHUB_TOKEN") or os.environ.get("GH_TOKEN") or ""
    if token:
        headers["Authorization"] = f"Bearer {token}"

    url = asset["browser_download_url"]
    data = None
    last_err: Exception | None = None
    for attempt in range(1, 6):
        try:
            req = urllib.request.Request(url, headers=headers)
            data = urllib.request.urlopen(req, timeout=300).read()
            break
        except Exception as e:  # noqa: BLE001 — retry network/403 in CI
            last_err = e
            print(f"download attempt {attempt} failed: {e}", flush=True)
            import time

            time.sleep(attempt * 2)
    if data is None:
        raise SystemExit(f"failed to download core asset: {last_err}")

    extract = Path("dist/core-extract")
    if extract.exists():
        shutil.rmtree(extract)
    extract.mkdir(parents=True)

    name = asset["name"].lower()
    if name.endswith(".tar.gz") or name.endswith(".tgz"):
        with tarfile.open(fileobj=io.BytesIO(data), mode="r:gz") as tf:
            tf.extractall(extract)
    else:
        with zipfile.ZipFile(io.BytesIO(data)) as zf:
            zf.extractall(extract)

    found: Path | None = None
    for p in extract.rglob("*"):
        if not p.is_file():
            continue
        low = p.name.lower()
        if low == core_bin.lower():
            found = p
            break
        if low in ("sing-box", "sing-box.exe") and found is None:
            found = p
    if found is None:
        for p in sorted(extract.rglob("*")):
            print(p)
        raise SystemExit("core binary not found in archive")

    pkg = Path("dist/package")
    pkg.mkdir(parents=True, exist_ok=True)

    # macOS .app layout: put core next to the executable inside Contents/MacOS
    # so InstallBundledCore / ResolveBinary find it without PATH setup.
    # Do NOT leave a loose sing-box next to the .app — users should drag only
    # Swell-Box.app into /Applications like any normal Mac app. On first run the
    # core is copied to ~/.swellbox/bin and no longer depends on the zip folder.
    macos_bin = pkg / "Swell-Box.app" / "Contents" / "MacOS"
    if macos_bin.is_dir():
        dest = macos_bin / core_bin
        shutil.copy2(found, dest)
        try:
            dest.chmod(0o755)
        except OSError:
            pass
        run_hint = (
            "2. Drag Swell-Box.app to Applications (or open from here)\n"
            "   Core is inside the .app; first launch installs it to ~/.swellbox/bin"
        )
    else:
        dest = pkg / core_bin
        shutil.copy2(found, dest)
        try:
            dest.chmod(0o755)
        except OSError:
            pass
        for dll in extract.rglob("*.dll"):
            shutil.copy2(dll, pkg / dll.name)
        run_hint = f"2. Run Swell-Box{client_ext}"

    (pkg / "README.txt").write_text(
        "\n".join(
            [
                f"Swell-Box — offline package ({platform})",
                "1. Extract this folder",
                run_hint,
                f"3. {core_bin} is bundled for offline use (no download needed)",
                "",
                "macOS install (recommended):",
                "  • Drag Swell-Box.app → Applications (same as Chrome / WeChat)",
                "  • Open from Launchpad / Applications — not required to keep the zip folder",
                "  • If “damaged”: xattr -cr /Applications/Swell-Box.app",
                "  • First open: right-click → Open (Gatekeeper)",
                "macOS no internet: turn System Proxy off, or networksetup -setwebproxystate Wi-Fi off",
                "  (also secureweb + socksfirewall). Then Start after importing nodes. Log: ~/.swellbox/logs/core.log",
                "Data: ~/.swellbox  (Windows: %USERPROFILE%\\.swellbox)",
                "",
            ]
        ),
        encoding="utf-8",
    )

    full = Path(f"dist/Swell-Box-{platform}-full.zip")
    if full.exists():
        full.unlink()
    with zipfile.ZipFile(full, "w", zipfile.ZIP_DEFLATED) as zf:
        for p in pkg.rglob("*"):
            if p.is_file():
                zf.write(p, p.relative_to(pkg).as_posix())
    print("wrote", full, "size", full.stat().st_size, flush=True)


if __name__ == "__main__":
    main()
