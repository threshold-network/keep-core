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

How a citation is identified
A citation is identified by where it sits in the SPEC, not by what it points at:
the spec file, the line in that file, and which citation on that line it is.
The source path and line number are values, not identity. That distinction is
what makes the check usable: a re-pin changes the number, so a key built from
the number would report every moved citation as new and could never pass.

Comparing a snapshot taken before the re-pin with one taken after it therefore
answers the question that matters: does each citation still name the same test?
A citation whose number moved but whose title did not is a correct re-pin. A
citation whose number stayed but whose title changed is a citation that now
points at a different test. A path change is reported separately, because a
citation that moves to another file is a deliberate act, not a drift.

Run from the verification directory:

    python3 tools/citation_drift.py <spec-root> <tbtc-v2 repo> <tbtc-v2 rev>
                                    --write <snapshot.json>
    python3 tools/citation_drift.py <spec-root> <tbtc-v2 repo> <tbtc-v2 rev>
                                    --baseline <snapshot.json>

Exit 1 when any citation names a different test than it did at the baseline.

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
# it("title", ...), describe("title", ...), context("title", ...) and the
# .only / .skip variants, with the title on the same line.
TITLE_SAME_LINE = re.compile(
    r"""^\s*(?:it|describe|context)(?:\.(?:only|skip))?\(\s*(['"`])(.+?)\1"""
)
# The same, but prettier wrapped the call so the title is on the next line:
#
#     it(
#       "some title",
#       async () => {
TITLE_WRAPPED = re.compile(
    r"""^\s*(?:it|describe|context)(?:\.(?:only|skip))?\(\s*$"""
)
# A bare string on the next non-blank line is the title.
QUOTED_LINE = re.compile(r"""^\s*(['"`])(.+?)\1""")


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
    """Return the title of the nearest enclosing it()/describe() at or above `number`.

    A citation usually lands on the `it(` line itself, but it may also land on a
    line inside the test body, in which case the enclosing test is the one above
    it. When the call is wrapped so that the title is on the following line, the
    title is taken from there.
    """
    for index in range(min(number, len(lines)) - 1, -1, -1):
        line = lines[index]
        match = TITLE_SAME_LINE.match(line)
        if match:
            return match.group(2)
        if TITLE_WRAPPED.match(line):
            for follow in range(index + 1, min(index + 4, len(lines))):
                if not lines[follow].strip():
                    continue
                quoted = QUOTED_LINE.match(lines[follow])
                if quoted:
                    return quoted.group(2)
                break
    return None


def collect(spec_root, repo, rev):
    """Map every pinned test-file citation to the test title it lands in.

    The key is the citation's place in the spec: the spec file, the spec line,
    and which citation on that line it is. The source path, the source number
    and the title are values.
    """
    entries = {}
    root = pathlib.Path(spec_root)
    for md in sorted(root.rglob("*.md")):
        text = md.read_text(encoding="utf-8", errors="replace")
        for line_no, line in enumerate(text.splitlines(), 1):
            for occurrence, match in enumerate(CITATION.finditer(line)):
                if not match.group(1):
                    continue
                path, first = match.group(2), int(match.group(3))
                key = f"{md.relative_to(root)}:{line_no}#{occurrence}"
                lines = blob(repo, rev, path)
                if lines is None:
                    entries[key] = {
                        "status": "absent", "path": path, "number": first,
                        "title": None,
                    }
                    continue
                title = nearest_title(lines, first)
                entries[key] = {
                    "status": "ok" if title else "no-title",
                    "path": path, "number": first, "title": title,
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
    moved = []
    added = []
    removed = []
    for key, value in sorted(entries.items()):
        old = before.get(key)
        if old is None:
            added.append(key)
            continue
        if old.get("title") != value.get("title"):
            drifted.append((key, old, value))
        if old.get("path") != value.get("path"):
            moved.append((key, old, value))
    for key in sorted(set(before) - set(entries)):
        removed.append(key)

    print(f"citations checked: {len(entries)}")
    print(f"drifted: {len(drifted)}")
    for key, old, new in drifted:
        print(f"  {key} ({old.get('path')}:{old.get('number')})")
        print(f"    was: {old.get('title')}")
        print(f"    now: {new.get('title')} ({new.get('path')}:{new.get('number')})")
    print(f"path changed: {len(moved)}")
    for key, old, new in moved:
        print(f"  {key}: {old.get('path')} -> {new.get('path')}")
    print(f"new citations: {len(added)}  gone: {len(removed)}")
    for key in added:
        print(f"  new: {key}")
    for key in removed:
        print(f"  gone: {key}")
    print("RESULT: FAIL" if drifted else "RESULT: PASS")
    return 1 if drifted else 0


if __name__ == "__main__":
    sys.exit(main())
