#!/usr/bin/env python3
"""Pick first non-draft release from dist/rel-list.json -> dist/rel.json"""
from __future__ import annotations

import json
from pathlib import Path


def main() -> None:
    arr = json.loads(Path("dist/rel-list.json").read_text(encoding="utf-8"))
    rel = next(x for x in arr if not x.get("draft"))
    Path("dist/rel.json").write_text(
        json.dumps(rel, ensure_ascii=False, indent=2),
        encoding="utf-8",
    )
    print("picked", rel.get("tag_name"), flush=True)


if __name__ == "__main__":
    main()
