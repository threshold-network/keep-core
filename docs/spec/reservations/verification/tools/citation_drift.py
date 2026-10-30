"""Report when a pinned citation no longer names the test it used to name.

`smoke.py` checks that a pinned citation resolves: the file exists at the pin and
the line is inside it. That is necessary but not sufficient. A line-number
citation is positional, so when the cited file shifts between pins the same
number can land on a different `it(` and still pass every check. The result is a
spec that cites a real line while claiming a test that line no longer contains.

This tool closes that gap by resolving each pinned citation to the NAME of the
test case it lands in, so a re-pin that moves a file shows up as a name change
instead of a silent drift. It is the check the pin rule cannot make: the pin rule
asks whether a citation names a revision, this asks whether it still says what
the row claims.

Run from the verification directory:

    python3 tools/citation_drift.py <spec-root> <tbtc-v2 repo> <tbtc-v2 rev>
                                    --baseline <snapshot.json>

The snapshot is taken at the current pin with `--write <snapshot.json>` and
compared after a re-pin. Exit 1 when any citation names a different test.

Only explicitly pinned citations to TypeScript or JavaScript test files are
checked: those are the ones whose line numbers move with an upstream edit and
whose meaning is carried by a test title. A bare citation is the pin rule's
business, not this tool's.
"""
import argparse
import json
import pathlib
import re
import subprocess
import sys

CITATION = re.compile(
    r"tbtc-v2(?:@([0-9a-f]{7,40}))?:"
    r"([A-Za-z0-9_./-]+\.(?:ts|js)):(\d+)(?:-(\d+))?"
)
TITLE = re.compile(r"""^\s*(?:it|describe|context)\(\s*(['"`])(.+?)\1""")


def blob(repo, rev, path):
    """Return the lines of `path` at `rev`, or None when the path is absent."""
    result = subprocess.run(
        ["git", "-C", repo, "show", f"{rev}:{path}"],
        capture_output=True, text=True,
    )
    if result.returncode != 0:
        return None
    return result.stdout.splitlines()


def nearest_title(lines, number):
    """Return the title of the nearest enclosing it()/describe() at or above `number`."""
    for index in range(min(number, len(lines)) - 1, -1, -1):
        match = TITLE.match(lines[index])
        if match:
            return match.group(2)
    return None


def collect(spec_root, repo, rev):
    """Map every pinned test-file citation to the test title it lands in."""
    entries = {}
    root = pathlib.Path(spec_root)
    for md in sorted(root.rglob("*.md")):
        text = md.read_text(encoding="utf-8", errors="replace")
        for line_no, line in enumerate(text.splitlines(), 1):
            for match in CITATION.finditer(line):
                if not match.group(1):
                    continue
                path, first = match.group(2), int(match.group(3))
                key = f"{md.relative_to(root)}:{line_no}|{path}:{first}"
                lines = blob(repo, rev, path)
                if lines is None:
                    entries[key] = {"status": "absent", "title": None}
                    continue
                entries[key] = {
                    "status": "ok" if nearest_title(lines, first) else "no-title",
                    "title": nearest_title(lines, first),
                }
    return entries


def main():
    parser = argparse.ArgumentParser(
        description="Report citations that no longer name the test they named.")
    parser.add_argument("spec_root", help="the reservations spec directory")
    parser.add_argument("tbtc_v2", help="tbtc-v2 checkout")
    parser.add_argument("revision", help="the revision to resolve citations at")
    parser.add_argument("--baseline", help="snapshot to compare against")
    parser.add_argument("--write", help="write a snapshot at this revision")
    args = parser.parse_args()

    entries = collect(args.spec_root, args.tbtc_v2, args.revision)

    if args.write:
        pathlib.Path(args.write).write_text(
            json.dumps(entries, indent=1, sort_keys=True))
        print(f"snapshot written: {args.write} ({len(entries)} citations)")
        return 0

    if not args.baseline:
        ok = sum(1 for v in entries.values() if v["status"] == "ok")
        print(f"citations checked: {len(entries)}  resolvable={ok}")
        for key, value in sorted(entries.items()):
            if value["status"] != "ok":
                print(f"  {value['status']}: {key}")
        return 0

    before = json.loads(pathlib.Path(args.baseline).read_text())
    drifted = []
    for key, value in sorted(entries.items()):
        old = before.get(key)
        if old is None:
            drifted.append((key, "not in baseline", value["title"]))
        elif old.get("title") != value.get("title"):
            drifted.append((key, old.get("title"), value.get("title")))

    print(f"citations checked: {len(entries)}")
    print(f"drifted: {len(drifted)}")
    for key, old, new in drifted:
        print(f"  {key}\n    was: {old}\n    now: {new}")
    print("RESULT: FAIL" if drifted else "RESULT: PASS")
    return 1 if drifted else 0


if __name__ == "__main__":
    sys.exit(main())
