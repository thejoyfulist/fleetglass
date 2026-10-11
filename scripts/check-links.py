#!/usr/bin/env python3
"""Check relative links in public Markdown files without network access."""
import re
import sys
from pathlib import Path
from urllib.parse import unquote, urlsplit

root = Path(__file__).resolve().parents[1]
files = [root / "README.md", *sorted((root / "docs").glob("*.md"))]
errors = []
for file in files:
    content = file.read_text()
    targets = re.findall(r"!?(?:\[[^\]]*\])\(([^)]+)\)", content)
    targets += re.findall(r'''(?:src|href)=["']([^"']+)["']''', content)
    for target in targets:
        target = target.strip().split()[0].strip("<>")
        url = urlsplit(target)
        if url.scheme or target.startswith("//") or target.startswith("mailto:"):
            continue
        if not url.path:
            continue
        path = (file.parent / unquote(url.path)).resolve()
        if not path.is_relative_to(root) or not path.exists():
            errors.append(f"{file.relative_to(root)}: {target}")
if errors:
    print("Broken local links:\n" + "\n".join(errors), file=sys.stderr)
    sys.exit(1)
print(f"Checked relative links in {len(files)} Markdown files")
