"""Check the verification set for review readiness.

Usage: python3 smoke.py <verification-dir> <tbtc-v2 repo|-> <keep-core repo>

Checks Markdown rendering, local links, explicit citations, table shape, the
500-line file limit, ASCII dashes and quotes, and that 11-traceability.md lists
exactly the canonical IDs. An explicit citation (repo:path:line or
repo@revision:path:line) must exist at its pin; PINS holds the default pins for
citations without a revision. Bare and relative citations are not checked
here; repin.py reports them. Requires markdown-it-py.

Partial mode: pass '-' as the tbtc-v2 repository to skip only the tbtc-v2
citations (no git access for that repository). The skipped citations are
counted in citations_skipped and are not part of
explicit_pinned_citations_checked. Every other check still runs.

Quote check (warnings only, never an error): a double-quoted string that is
unambiguously adjacent to an explicit citation in the same paragraph, table
cell or list item is searched for in the cited source lines, in either order
(quote before citation, or citation before quote). Whitespace is collapsed and
Unicode dashes and quotes in the source are converted to ASCII. A "..." inside
a quote matches any text between its fragments. Quotes separated from a
citation by words, a sentence end or a semicolon are not associated; they are
only counted in quotes_unassociated. A mismatch is reported in warnings and
quote_warnings. The quote check never replaces or hides a source, link, table
or traceability error.
"""
import argparse
import collections
import json
import pathlib
import re
import subprocess
import sys
from markdown_it import MarkdownIt

PINS = {"tbtc-v2": "9d2086965", "keep-core": "f99ef96e3"}
citations = re.compile(r"(tbtc-v2|keep-core)(?:@([0-9a-f]{7,40}))?:([A-Za-z0-9_./-]+\.(?:sol|go|ts|md|json|cjs|yml|adoc)|(?:[A-Za-z0-9_./-]+/)?Makefile):(\d+)(?:-(\d+))?")
ids = re.compile(r"((?:SM|PAR|INV|US|EC|FM|DR|TM|XR|AR)-[A-Z0-9]+(?:-[A-Z0-9]+)*)")
quotes = re.compile(r'"([^"]+)"')
code_spans = re.compile(r"`[^`]*`")
ascii_map = str.maketrans({"\u2013": "-", "\u2014": "-", "\u2018": "'", "\u2019": "'", "\u201c": '"', "\u201d": '"'})
TIGHT_WORDS = {"", "on", "at", "in"}
ALT_WORDS = {"/", "or", "and"}


def squash(text):
    return re.sub(r"\s+", " ", text.translate(ascii_map)).strip()


def quote_in_source(quote, source):
    fragments = [squash(part) for part in re.split(r"\.\.\.|\u2026", quote.replace("\\|", "|"))]
    fragments = [part for part in fragments if part]
    if not fragments:
        return False
    haystack = squash(source)
    position = 0
    for fragment in fragments:
        found = haystack.find(fragment, position)
        if found < 0:
            return False
        position = found + len(fragment)
    return True


def link_kind(glue):
    """Classify the text between two neighbours: tight, alt, or far (not associated)."""
    if re.search(r";|\.(?:\s|$)", glue):
        return "far"
    word = re.sub(r"[\s`()\[\],:]+", "", glue).lower()
    if word in TIGHT_WORDS:
        return "tight"
    if word in ALT_WORDS:
        return "alt"
    return "far"


def associate(content):
    """Return (quote group, citation groups, unassociated quote count) for one inline run.

    A quote group is a list of alternate quote strings ("A" or "B" cite); a
    citation group is a list of citation matches written together. A quote
    group is associated only with the citation group directly touching it.
    """
    flat = content.replace("\n", " ")
    masked = code_spans.sub(lambda m: " " * len(m.group()), flat)
    cites = list(citations.finditer(flat))
    items = [("C", m.start(), m.end(), m) for m in cites]
    for m in quotes.finditer(masked):
        if any(m.start() < c.start() < m.end() for c in cites):
            continue
        items.append(("Q", m.start(), m.end(), flat[m.start() + 1:m.end() - 1]))
    items.sort(key=lambda item: item[1])

    groups = []
    last = None
    for item in items:
        if last is not None and last[0] == item[0]:
            kind = link_kind(flat[last[2]:item[1]])
            joins = kind == "alt" or (kind == "tight" and item[0] == "C")
            if joins:
                groups[-1]["items"].append(item)
                last = item
                continue
        groups.append({"type": item[0], "items": [item]})
        last = item

    tight = {}
    for index, group in enumerate(groups[:-1]):
        nxt = groups[index + 1]
        if group["type"] != nxt["type"]:
            end = group["items"][-1][2]
            start = nxt["items"][0][1]
            if link_kind(flat[end:start]) == "tight":
                tight.setdefault(index, []).append(index + 1)
                tight.setdefault(index + 1, []).append(index)

    candidates = {i: list(tight.get(i, [])) for i, g in enumerate(groups) if g["type"] == "Q"}
    changed = True
    while changed:
        changed = False
        claimed = {c[0] for c in candidates.values() if len(c) == 1}
        for q, cands in candidates.items():
            if len(cands) > 1:
                rest = [c for c in cands if c not in claimed]
                if rest and len(rest) < len(cands):
                    candidates[q] = rest
                    changed = True

    pairs = []
    unassociated = 0
    for q, cands in candidates.items():
        if not cands:
            unassociated += len(groups[q]["items"])
            continue
        alternates = [item[3] for item in groups[q]["items"]]
        cited = [item[3] for c in cands for item in groups[c]["items"]]
        pairs.append((alternates, cited))
    return pairs, unassociated


def main():
    parser = argparse.ArgumentParser(description="Check the reservations verification set.")
    parser.add_argument("verification")
    parser.add_argument("tbtc_v2", help="tbtc-v2 repository, or '-' to skip tbtc-v2 citations")
    parser.add_argument("keep_core")
    args = parser.parse_args()

    root = pathlib.Path(args.verification).resolve()
    repos = {"keep-core": args.keep_core}
    skipped_repos = []
    if args.tbtc_v2 == "-":
        skipped_repos.append("tbtc-v2")
    else:
        repos["tbtc-v2"] = args.tbtc_v2
    cache = {}
    errors = []
    warnings = []
    renderer = MarkdownIt("commonmark", {"html": False}).enable("table")

    def blob(repo, rev, path):
        key = (repo, rev, path)
        if key not in cache:
            result = subprocess.run(["git", "-C", repos[repo], "show", f"{rev}:{path}"], capture_output=True, text=True)
            value = result.stdout if result.returncode == 0 else None
            if value is None and "/" not in path:
                names = subprocess.run(["git", "-C", repos[repo], "ls-tree", "-r", "--name-only", rev], capture_output=True, text=True).stdout.splitlines()
                candidates = [name for name in names if name.endswith("/" + path)]
                if len(candidates) == 1:
                    value = blob(repo, rev, candidates[0])
            cache[key] = value
        return cache[key]

    def cited_text(match):
        """Return (status, text): status is skipped, absent, range or ok."""
        repo, rev, source_path, start, end = match.groups()
        if repo not in repos:
            return "skipped", None
        value = blob(repo, rev or PINS[repo], source_path)
        if value is None:
            return "absent", None
        lines = value.splitlines()
        if int(end or start) > len(lines):
            return "range", None
        return "ok", "\n".join(lines[int(start) - 1:int(end or start)])

    source_cites = 0
    skipped_cites = 0
    quote_counts = collections.Counter()
    rendered_bytes = 0
    links = 0
    head_definitions = collections.defaultdict(list)
    table_definitions = set()
    markdown_files = sorted(root.glob("*.md"))
    for path in markdown_files:
        text = path.read_text()
        if len(text.splitlines()) > 500:
            errors.append(f"{path.name}: exceeds 500 lines")
        if re.search(r"^(<<<<<<<|=======|>>>>>>>)", text, re.M):
            errors.append(f"{path.name}: merge marker")
        if re.search(r"[\u2013\u2014\u2018\u2019\u201c\u201d]", text):
            errors.append(f"{path.name}: non-ASCII dash/quote")
        rendered = renderer.render(text)
        rendered_bytes += len(rendered.encode())
        for token in renderer.parse(text):
            if token.type != "inline":
                continue
            for child in token.children or []:
                if child.type == "link_open":
                    href = child.attrGet("href") or ""
                    if "://" in href or href.startswith("#"):
                        continue
                    target = (path.parent / href.split("#", 1)[0]).resolve()
                    links += 1
                    if not target.exists():
                        errors.append(f"{path.name}: missing link {href}")
            pairs, unassociated = associate(token.content)
            quote_counts["unassociated"] += unassociated
            for alternates, cited in pairs:
                resolved = [(match, *cited_text(match)) for match in cited]
                usable = [(match, source) for match, status, source in resolved if status == "ok"]
                if not usable:
                    kind = "skipped" if all(status == "skipped" for _, status, _ in resolved) else "unverifiable"
                    quote_counts[kind] += len(alternates)
                    continue
                for quote in alternates:
                    quote_counts["checked"] += 1
                    if any(quote_in_source(quote, source) for _, source in usable):
                        quote_counts["matched"] += 1
                    else:
                        where = ", ".join(match[0] for match, _ in usable)
                        warnings.append(f"{path.name}: quote not found in cited lines: {quote!r} vs {where}")
        width = None
        for number, line in enumerate(text.splitlines(), 1):
            if not line.startswith("|"):
                width = None
            else:
                cells = re.split(r"(?<!\\)\|", line.strip())[1:-1]
                if width is None:
                    width = len(cells)
                elif len(cells) != width:
                    errors.append(f"{path.name}:{number}: table has {len(cells)} cells; expected {width}")
            if re.match(r"(?:0[1-9]|10)-", path.name):
                heading = re.match(r"^#{2,4}\s+" + ids.pattern + r"\b", line)
                if heading:
                    head_definitions[heading[1]].append(path.name)
                row = re.match(r"^\|\s*" + ids.pattern + r"\s*\|", line)
                if row:
                    table_definitions.add(row[1])
        for match in citations.finditer(text):
            status, _ = cited_text(match)
            if status == "skipped":
                skipped_cites += 1
                continue
            source_cites += 1
            if status == "absent":
                errors.append(f"{path.name}: absent source {match[0]}")
            elif status == "range":
                errors.append(f"{path.name}: source line exceeds file {match[0]}")

    for key, owners in head_definitions.items():
        if len(owners) != 1:
            errors.append(f"{key}: duplicate heading owners {owners}")
    expected = set(head_definitions) | table_definitions
    matrix = (root / "11-traceability.md").read_text()
    matrix_ids = re.findall(r"^\| \[(" + ids.pattern + r")\]", matrix, re.M)
    matrix_keys = [pair[0] for pair in matrix_ids]
    if set(matrix_keys) != expected:
        errors.append("traceability does not cover exactly the canonical IDs: missing=" + str(sorted(expected - set(matrix_keys))) + "; extra=" + str(sorted(set(matrix_keys) - expected)))
    if len(matrix_keys) != len(set(matrix_keys)):
        errors.append("traceability has duplicate IDs")
    report = {
        "documents_rendered": len(markdown_files),
        "rendered_html_bytes": rendered_bytes,
        "local_links_checked": links,
        "explicit_pinned_citations_checked": source_cites,
        "citations_skipped": skipped_cites,
        "skipped_repositories": skipped_repos,
        "quotes_checked": quote_counts["checked"],
        "quotes_matched": quote_counts["matched"],
        "quote_warnings": len(warnings),
        "quotes_skipped": quote_counts["skipped"],
        "quotes_unverifiable": quote_counts["unverifiable"],
        "quotes_unassociated": quote_counts["unassociated"],
        "canonical_review_ids": len(expected),
        "errors": errors,
        "warnings": warnings,
    }
    print(json.dumps(report, indent=2))
    sys.exit(1 if errors else 0)


main()
