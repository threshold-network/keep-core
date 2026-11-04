"""Fail when a citation written in new or changed lines carries no explicit pin.

A bare citation (`Bridge.sol:2116`) or a repository-qualified citation without a
revision (`tbtc-v2:solidity/contracts/bridge/Bridge.sol:2116`) names its line at the
default pin. When that file changed between the previous freeze pin and the current
one, the number names different code depending on which pin a reader assumes. The
2026-10-30 re-pin repaired that by hand in 18 places, and the review found 1,487 more
sites where a mechanical check cannot tell a stale number from a correct one. The set
therefore does not re-read what it already published; it refuses to grow the defect.

The rule applies to lines added or changed since the baseline commit recorded in
`pin-rule.json`. Citations that were in the set at the baseline are not checked. A
relative citation (`:900`) inherits the file named by its owner in the same row, list
item or paragraph, so it is checked through that owner.

Run from the verification directory:

    python3 tools/pin_rule.py <keep-core repo> <tbtc-v2 repo> [--baseline SHA]
"""
import argparse
import json
import pathlib
import sys

import repin

CONFIG = pathlib.Path(__file__).with_name("pin-rule.json")


def load(path):
    with open(path, encoding="utf-8") as handle:
        return json.load(handle)


def changed_files(repo, previous, current):
    listing = repin.git(repo, "diff", "--name-only", "--no-color", previous, current)
    if listing is None:
        raise SystemExit(f"pin rule: cannot diff {previous}..{current} in {repo}")
    return [line for line in listing.splitlines() if line.strip()]


def added_lines(repo, baseline, paths):
    """Return (path, line number, text) for every added markdown line since the baseline.

    Working-tree only: a file that is not yet tracked is read whole, so a citation
    added locally is checked before it is committed. Only `.md` files are read; the
    tools' own sources and their example citations are not evidence.
    """
    diff = repin.git(repo, "diff", "-U0", "--no-color", "--no-ext-diff", baseline, "--", *paths)
    if diff is None:
        raise SystemExit(f"pin rule: cannot diff {baseline} against {repo}")
    found, current, number = [], None, 0
    for line in diff.splitlines():
        if line.startswith("+++ "):
            target = line[4:].strip()
            current = None if target == "/dev/null" else (target[2:] if target.startswith("b/") else target)
            if current is not None and not current.endswith(".md"):
                current = None
            continue
        hunk = repin.HUNK.match(line)
        if hunk:
            number = int(hunk.group(3))
            continue
        if current is None:
            continue
        if line.startswith("+"):
            found.append((current, number, line[1:]))
            number += 1
    untracked = (repin.git(repo, "ls-files", "--others", "--exclude-standard", "--", *paths) or "").splitlines()
    for name in untracked:
        if not name.endswith(".md"):
            continue
        text = (pathlib.Path(repo) / name).read_text(encoding="utf-8", errors="replace").splitlines()
        found.extend((name, index, line) for index, line in enumerate(text, 1))
    return found


def candidates(names, token):
    """Changed files a citation token could name: exact, or one unique suffix match."""
    exact = [name for name in names if name == token["name"]]
    if exact:
        return exact
    return [name for name in names if name.endswith("/" + token["name"])]


def violations(lines, changed):
    found = []
    for path, number, text in lines:
        for _, _, token in repin.tokens(text):
            if token.get("relative") or token.get("rev"):
                continue
            hits = candidates(changed.get(token["repo"], []), token)
            if hits:
                found.append(dict(file=path, line=number, citation=text.strip(), repo=token["repo"], target=hits))
    return found


def main():
    parser = argparse.ArgumentParser(description="Check the pin rule for new citations.")
    parser.add_argument("keep_core", help="keep-core checkout that holds this set")
    parser.add_argument("tbtc_v2", help="tbtc-v2 checkout, for the contract revision range")
    parser.add_argument("--baseline", help="override the baseline commit recorded in pin-rule.json")
    args = parser.parse_args()

    config = load(CONFIG)
    baseline = args.baseline or config["baseline"]
    repos = {"tbtc-v2": args.tbtc_v2, "keep-core": args.keep_core}

    changed, ranges = {}, []
    for repo, pins in config["pins"].items():
        changed[repo] = changed_files(repos[repo], pins["previous"], pins["current"])
        ranges.append(f"{repo} {pins['previous']}..{pins['current']} ({len(changed[repo])} files changed)")
    print(f"pin rule: baseline {baseline}")
    for entry in ranges:
        print(f"  {entry}")

    lines = added_lines(args.keep_core, baseline, config["paths"])
    print(f"checked: {len(lines)} lines added or changed since the baseline")
    found = violations(lines, changed)
    for item in found:
        targets = ", ".join(item["target"][:2]) + (f" and {len(item['target']) - 2} more" if len(item["target"]) > 2 else "")
        print(f"  {item['file']}:{item['line']}: {item['citation']} names {targets}, which changed in this range; add @{config['pins'][item['repo']]['current']}")
    print(f"violations: {len(found)}")
    print("RESULT: FAIL" if found else "RESULT: PASS")
    return 1 if found else 0


if __name__ == "__main__":
    sys.exit(main())