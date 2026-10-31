"""Move the specification's line citations from one source commit to another.

Usage:
  python3 repin.py <spec-dir> <tbtc-v2|keep-core> <repo-path> <old-rev> <new-rev> [--default] [--write]

Without --default, citations pinned as <repo>@<old-rev> move to <new-rev>.
With --default, citations that carry no revision move; <old-rev> must be the
repository's current default pin (PINS in smoke.py).

Citation forms:
  explicit  tbtc-v2:solidity/contracts/bridge/Reservation.sol:887 (a path that
            ends in Makefile is accepted too)
  bare      Reservation.sol:887 or test/bridge/Reservation.test.ts:146
            (.sol and .ts are tbtc-v2, .go is keep-core; a partial path
            resolves by a unique suffix match)
  relative  `:900`, which uses the file most recently named in the same table
            row, list item or paragraph, with or without a line number

A citation moves only when every cited line has the same text at <new-rev>.
A citation that cannot move is pinned explicitly to <old-rev>, so that it
still points at the code that it describes, and it is listed for review.
A relative citation, or a bare one that repeats an explicitly pinned file,
follows the citation that names its file. Because a rewritten citation can
change the file that later citations inherit, the helper re-reads its own
output and writes each citation that no longer resolves to its intended
commit, path and lines in full explicit form (made_explicit in the report).
11-traceability.md is derived and is skipped; rebuild it with traceability.py
after a write.

Nothing is written without --write. --write refuses to run while a selected
citation does not resolve to exactly one file, or while a relative citation
has no file named in its own row, list item or paragraph.
"""
import json
import pathlib
import re
import subprocess
import sys

EXPLICIT = re.compile(r"(tbtc-v2|keep-core)(?:@([0-9a-f]{7,40}))?:([A-Za-z0-9_./-]+\.(?:sol|go|ts|md|json|cjs|yml|adoc)|(?:[A-Za-z0-9_./-]+/)?Makefile)(?::(\d+)(?:-(\d+))?)?")
BARE = re.compile(r"(?<![A-Za-z0-9_./@:-])([A-Za-z0-9_-]+(?:[./][A-Za-z0-9_-]+)*\.(sol|go|ts))(?::(\d+)(?:-(\d+))?)?(?![A-Za-z0-9_-])")
RELATIVE = re.compile(r"(?<![A-Za-z0-9_./@:\[-]):(\d+)(?:-(\d+))?(?![\d:])")
REPO_BY_EXTENSION = {"sol": "tbtc-v2", "ts": "tbtc-v2", "go": "keep-core"}
HUNK = re.compile(r"^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@", re.M)
COMMIT = re.compile(r"[0-9a-f]{7,40}")
DERIVED = "11-traceability.md"


def git(repo_path, *args):
    result = subprocess.run(["git", "-C", repo_path, *args], capture_output=True, text=True)
    return result.stdout if result.returncode == 0 else None


class Source:
    def __init__(self, path):
        self.path, self.revs, self.files, self.trees, self.hunks = path, {}, {}, {}, {}

    def full(self, rev):
        if rev not in self.revs:
            out = git(self.path, "rev-parse", "--verify", "--quiet", rev + "^{commit}")
            self.revs[rev] = out.strip() if out else None
        return self.revs[rev]

    def lines(self, rev, path):
        if (rev, path) not in self.files:
            out = git(self.path, "show", f"{rev}:{path}")
            self.files[rev, path] = None if out is None else out.splitlines()
        return self.files[rev, path]

    def resolve(self, rev, name):
        if self.lines(rev, name) is not None:
            return name
        if rev not in self.trees:
            self.trees[rev] = (git(self.path, "ls-tree", "-r", "--name-only", rev) or "").splitlines()
        found = [path for path in self.trees[rev] if path.endswith("/" + name)]
        return found[0] if len(found) == 1 else None

    def changes(self, old, new, path):
        if (old, new, path) not in self.hunks:
            out = git(self.path, "diff", "-U0", "--no-color", "--no-ext-diff", old, new, "--", path) or ""
            self.hunks[old, new, path] = [(int(a), int(b or 1), int(d or 1)) for a, b, _, d in HUNK.findall(out)]
        return self.hunks[old, new, path]


def map_line(hunks, line):
    shift = 0
    for start, removed, added in hunks:
        if removed == 0:
            if start >= line:
                break
            shift += added
            continue
        if line < start:
            break
        if line < start + removed:
            return None
        shift += added - removed
    return line + shift


def blocks(lines):
    block = []
    for number, line in enumerate(lines, 1):
        boundary = not line.strip() or line.startswith(("|", "#")) or re.match(r"(?:[-*]|\d+\.)\s", line)
        if boundary and block:
            yield block
            block = []
        if line.strip():
            block.append((number, line))
            if line.startswith(("|", "#")):
                yield block
                block = []
    if block:
        yield block


def tokens(line):
    found = [(m.start(), m.end(), dict(repo=m[1], rev=m[2], name=m[3], first=m[4], last=m[5], explicit=True)) for m in EXPLICIT.finditer(line)]
    for pattern in (BARE, RELATIVE):
        taken = [(start, end) for start, end, _ in found]
        for m in pattern.finditer(line):
            if any(m.start() < end and m.end() > start for start, end in taken):
                continue
            if pattern is BARE:
                token = dict(repo=REPO_BY_EXTENSION[m[2]], rev=None, name=m[1], first=m[3], last=m[4], explicit=False)
            else:
                token = dict(relative=True, first=m[1], last=m[2])
            found.append((m.start(), m.end(), token))
    return sorted(found, key=lambda item: item[0])


def same_file(owner, token):
    return (
        owner is not None and not token["explicit"] and owner["rev"] and owner["repo"] == token["repo"]
        and pathlib.PurePosixPath(owner["path"]).name == pathlib.PurePosixPath(token["name"]).name
    )


def references(text):
    refs, loose = [], []
    for block in blocks(text.splitlines()):
        anchor = None
        for number, line in block:
            for start, end, token in tokens(line):
                if token.get("relative"):
                    if anchor is None:
                        loose.append(f"{number}: {line[start:end]}")
                        continue
                    ref = dict(refs[anchor], kind="relative", anchor=anchor)
                else:
                    owner = refs[anchor] if anchor is not None else None
                    if same_file(owner, token):
                        ref = dict(owner, anchor=anchor, name=token["name"])
                    else:
                        ref = dict(repo=token["repo"], rev=token["rev"], path=token["name"], name=token["name"], anchor=len(refs))
                    ref["kind"] = ("explicit" if token["explicit"] else "bare") if token["first"] else "mention"
                ref.update(line=number, start=start, end=end, text=line[start:end], ranged=token["last"] is not None)
                ref["first"] = int(token["first"]) if token["first"] else None
                ref["last"] = int(token["last"] or token["first"]) if token["first"] else None
                refs.append(ref)
                if ref["anchor"] == len(refs) - 1:
                    anchor = ref["anchor"]
    return refs, loose


def span(first, last, ranged):
    return f"{first}-{last}" if ranged else f"{first}"


def target_of(source, ref, default_commit):
    commit = source.full(ref["rev"]) if ref["rev"] else default_commit
    if commit is None or ref["first"] is None:
        return None
    return commit, source.resolve(commit, ref["path"]), ref["first"], ref["last"]


def render(text, refs, replacements):
    lines = text.split("\n")
    for index in sorted(replacements, key=lambda i: (refs[i]["line"], refs[i]["start"]), reverse=True):
        ref = refs[index]
        row = lines[ref["line"] - 1]
        lines[ref["line"] - 1] = row[:ref["start"]] + replacements[index] + row[ref["end"]:]
    return "\n".join(lines)


def main():
    args = [arg for arg in sys.argv[1:] if not arg.startswith("--")]
    flags = {arg for arg in sys.argv[1:] if arg.startswith("--")}
    if len(args) != 5 or args[1] not in REPO_BY_EXTENSION.values() or flags - {"--default", "--write"}:
        sys.exit(__doc__)
    spec_dir, repo_name, repo_path, old, new = args
    source = Source(repo_path)
    old_full, new_full = source.full(old), source.full(new)
    if not (COMMIT.fullmatch(old) and COMMIT.fullmatch(new) and old_full and new_full):
        sys.exit(f"old-rev and new-rev must be commit hashes present in {repo_path}")
    default = "--default" in flags
    labels = {old_full: old, new_full: new}

    def explicit(target, ranged):
        commit, full_path, first, last = target
        pin = "" if default and commit == new_full else "@" + labels[commit]
        return f"{repo_name}{pin}:{full_path}:{span(first, last, ranged)}"

    report = {
        "selected": 0, "moved": 0, "unchanged": 0, "pinned_to_old": [], "kept_at_old": [],
        "made_explicit": [], "unresolved": [], "relative_without_file": [],
    }
    edits = {}
    for path in sorted(pathlib.Path(spec_dir).glob("*.md")):
        if path.name == DERIVED:
            continue
        text = path.read_text()
        refs, loose = references(text)
        report["relative_without_file"] += [f"{path.name}:{item}" for item in loose]
        outcome, replacements, intended = {}, {}, {}
        for index, ref in enumerate(refs):
            if ref["repo"] != repo_name:
                continue
            where = f"{path.name}:{ref['line']}: {ref['text']}"
            selected = (ref["rev"] is None) == default and (default or source.full(ref["rev"]) == old_full)
            if not selected:
                target = target_of(source, ref, None)
                if target and target[1]:
                    labels.setdefault(target[0], ref["rev"])
                    intended[index] = target
                continue
            full_path = source.resolve(old_full, ref["path"])
            if ref["kind"] == "mention":
                if ref["anchor"] == index:
                    outcome[index] = "moved" if full_path else "unresolved"
                    if full_path and ref["rev"]:
                        replacements[index] = f"{repo_name}@{new}:{ref['name']}"
                continue
            report["selected"] += 1
            if ref["anchor"] != index and outcome.get(ref["anchor"]) != "moved":
                outcome[index] = "unresolved" if outcome.get(ref["anchor"]) == "unresolved" else "kept"
                report["unresolved" if outcome[index] == "unresolved" else "kept_at_old"].append(where)
                if outcome[index] == "kept":
                    intended[index] = (old_full, full_path, ref["first"], ref["last"])
                continue
            old_lines = source.lines(old_full, full_path) if full_path else None
            if old_lines is None or ref["last"] > len(old_lines) or ref["first"] > ref["last"]:
                outcome[index] = "unresolved"
                report["unresolved"].append(where)
                continue
            new_lines = source.lines(new_full, full_path)
            hunks = source.changes(old_full, new_full, full_path) if new_lines is not None else None
            moved = [map_line(hunks, line) for line in range(ref["first"], ref["last"] + 1)] if hunks is not None else [None]
            if None in moved or any(b - a != 1 for a, b in zip(moved, moved[1:])) or any(
                target > len(new_lines) or old_lines[line - 1] != new_lines[target - 1]
                for line, target in zip(range(ref["first"], ref["last"] + 1), moved)
            ):
                intended[index] = (old_full, full_path, ref["first"], ref["last"])
                if ref["kind"] == "explicit" and ref["rev"]:
                    outcome[index] = "kept"
                    report["kept_at_old"].append(where)
                    continue
                outcome[index] = "pinned"
                replacements[index] = explicit(intended[index], ref["ranged"])
                report["pinned_to_old"].append(f"{where} -> {replacements[index]}")
                continue
            outcome[index] = "moved"
            intended[index] = (new_full, full_path, moved[0], moved[-1])
            numbers = span(moved[0], moved[-1], ref["ranged"])
            report["moved" if moved[0] != ref["first"] else "unchanged"] += 1
            if ref["kind"] == "explicit":
                replacement = f"{repo_name}{'@' + new if ref['rev'] else ''}:{ref['name']}:{numbers}"
            elif ref["kind"] == "bare":
                replacement = f"{ref['name']}:{numbers}"
            else:
                replacement = f":{numbers}"
            if replacement != ref["text"]:
                replacements[index] = replacement
        # A rewritten citation can change the file context that later relative
        # or bare citations inherit. Re-read the output and make each citation
        # that no longer resolves to its intended commit, path and lines fully
        # explicit, which does not depend on context.
        after = new_full if default else None
        rewritten = render(text, refs, replacements)
        for _ in range(len(intended) + 1):
            check, _ = references(rewritten)
            if len(check) != len(refs):
                report["unresolved"].append(f"{path.name}: the rewrite changed the citation structure")
                break
            wrong = [i for i, want in intended.items() if target_of(source, check[i], after) != want]
            fixable = [i for i in wrong if replacements.get(i) != explicit(intended[i], refs[i]["ranged"])]
            if not wrong:
                break
            if not fixable:
                report["unresolved"] += [f"{path.name}:{refs[i]['line']}: {refs[i]['text']} (context not preserved)" for i in wrong]
                break
            for i in fixable:
                replacements[i] = explicit(intended[i], refs[i]["ranged"])
                report["made_explicit"].append(f"{path.name}:{refs[i]['line']}: {refs[i]['text']} -> {replacements[i]}")
            rewritten = render(text, refs, replacements)
        if rewritten != text:
            edits[path] = rewritten
    report["files_to_change"] = len(edits)
    print(json.dumps(report, indent=2))
    if "--write" not in flags:
        return 0
    if report["unresolved"] or report["relative_without_file"]:
        print("not written: make the unresolved and relative_without_file citations explicit first", file=sys.stderr)
        return 1
    for path, rewritten in edits.items():
        path.write_text(rewritten)
    print(f"written; rebuild {DERIVED} with traceability.py", file=sys.stderr)
    if default:
        print(f"then set PINS['{repo_name}'] in smoke.py and the README evidence pins to {new}", file=sys.stderr)
    return 0


if __name__ == "__main__":
    sys.exit(main())
