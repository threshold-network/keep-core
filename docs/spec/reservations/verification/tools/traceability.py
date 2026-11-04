"""Derive 11-traceability.md from the owning chapters, or check that it matches.

Usage: python3 traceability.py <verification-dir> [--check]

--check exits 1 on a structural problem (duplicate owner, owner without a
status, summary row that disagrees with its owner, an ID that no chapter owns,
no owned ID at all, a KC- entry whose Critical field is missing or is neither
yes nor no, a critical entry with no checker) or when 11-traceability.md
differs from the derivation. The critical rule: an entry is critical when its
Critical field starts with yes, and then its Checkers field must have real
text (not blank, none, n/a, missing or tbd). When the directory has no
11-traceability.md the matrix comparison is skipped. It also resolves the Go
checks (Test/Fuzz/Benchmark functions and *_test.go files) named in the
evidence fields (Tests/Rehearsed) and in the Checkers and Fails on fields
against the git checkout that holds the verification set, and lists those
that are not on that branch under check_references.missing_by_id. A missing
check does not change the exit code. In the reservations set the owning entry
pins its evidence to the commit that holds it, and smoke.py fails if that
pinned citation is absent; the keep-core set has no pinned citations and no
smoke run, so a missing Go check there is visible only in
check_references.missing_by_id.
"""
import collections
import difflib
import json
import pathlib
import re
import subprocess
import sys

ID = r"(?:SM|PAR|INV|US|EC|FM|DR|TM|XR|AR|KC)-[A-Z0-9]+(?:-[A-Z0-9]+)*"
ID_ONLY = re.compile(ID)
ID_ANY = re.compile(r"\b" + ID + r"\b")
STATUS = re.compile(r"\b(holds-untested|holds|violated|design-only|unknown)\b")
RANK = {"violated": 0, "unknown": 1, "design-only": 2, "holds-untested": 3, "holds": 4}
COLUMNS = ["holds", "holds-untested", "design-only", "unknown", "violated", "model", "pointer"]
CHAPTERS = [
    ("01", "System model"), ("02", "Invariants"), ("03", "Parameters and governance"),
    ("04", "User stories"), ("05", "Edge cases"), ("06", "Failure modes"),
    ("07", "Disaster recovery"), ("08", "Threat model"), ("09", "Cross-repo contract"),
    ("10", "Accepted risks"),
]
HEADING = re.compile(r"^(#{2,4})\s+(" + ID + r")\b\s*(.*)$")
FIELD = re.compile(
    r"^[ \t]*(?:- )?(?:\*\*)?(Status|Tests?|Rehearsed|Checkers|Fails on|Critical|Exposure)(?:\s*\([^)\n]*\))?(?:\*\*)?\s*[.:](?:\*\*)?[ \t]*",
    re.M,
)
FIELD_END = re.compile(r"\n[ \t]*\n|\n- |\n#|\n\|")
TEST_REFERENCE = re.compile(r"\.test\.ts|_test\.go|\bTest[A-Z]\w+|harness|rehears|\btests?\b|pinned by", re.I)
GO_CHECK = re.compile(r"\b(?:Test|Fuzz|Benchmark)[A-Z]\w*")
GO_TEST_FILE = re.compile(r"(?<![\w./-])((?:[\w-]+/)*[\w-]+_test\.go)\b")
GO_DEFINITION = re.compile(r"^func ((?:Test|Fuzz|Benchmark)\w*)\(", re.M)


def one_line(text):
    return re.sub(r"\s+", " ", text).strip()


def field_text(body, start):
    match = FIELD_END.search(body, start)
    return body[start:match.start() if match else len(body)].strip()


def cells(line):
    return [cell.strip() for cell in re.split(r"(?<!\\)\|", line.strip())[1:-1]]


def tokens_after_status(text):
    found = []
    for match in re.finditer(r"\bStatus\b", text):
        found += STATUS.findall(text[match.start():].split("\n", 1)[0])
    return found


def grade(found, status_text, descriptive):
    if found:
        return min(found, key=RANK.get), True
    if status_text and ID_ANY.search(status_text):
        return "pointer", True
    return ("model", True) if descriptive else ("unknown", False)


def heading_entries(path):
    lines = path.read_text().splitlines()
    result = []
    for index, line in enumerate(lines):
        match = HEADING.match(line)
        if not match:
            continue
        depth = len(match[1])
        end = len(lines)
        for later in range(index + 1, len(lines)):
            other = re.match(r"^(#{1,6})\s", lines[later])
            if other and len(other[1]) <= depth:
                end = later
                break
        body = "\n".join(lines[index + 1:end])
        found, evidence, status_text = [], [], ""
        fields = {}
        for field in FIELD.finditer(body):
            name, text = field[1], field_text(body, field.end())
            if name == "Status":
                found += STATUS.findall(text)
                status_text += " " + text
            elif name in ("Tests", "Test", "Rehearsed"):
                evidence.append(text)
            else:
                fields[name] = text
        if not found:
            found = tokens_after_status(body)
        value, explicit = grade(found, status_text, False)
        result.append({
            "id": match[2], "file": path.name, "line": index + 1,
            "title": one_line(match[3]).strip(" -"), "grade": value, "explicit": explicit,
            "evidence": one_line(" / ".join(evidence) or (status_text if value == "pointer" else "")),
            "checkers": fields.get("Checkers", ""),
            "fails_on": fields.get("Fails on", ""),
            "critical": fields.get("Critical", ""),
            "exposure": fields.get("Exposure", ""),
        })
    return result


def table_rows(path):
    lines = path.read_text().splitlines()
    result = []
    header = None
    for index, line in enumerate(lines):
        if not line.startswith("|"):
            header = None
            continue
        row = cells(line)
        if header is None:
            header = row
            continue
        if not row or not ID_ONLY.fullmatch(row[0]):
            continue
        status_cells = [cell for name, cell in zip(header, row) if re.search(r"status", name, re.I)]
        test_cells = [cell for name, cell in zip(header, row) if re.search(r"test|rehears", name, re.I)]
        found = [token for cell in status_cells for token in STATUS.findall(cell)]
        embedded = ""
        if not status_cells:
            joined = " ".join(row[1:])
            position = re.search(r"\bStatus\b", joined)
            if position:
                embedded = joined[position.start():]
                found = STATUS.findall(embedded)
        status_text = " ".join(status_cells) or embedded
        evidence = " / ".join(cell for cell in test_cells if cell)
        if not evidence and TEST_REFERENCE.search(status_text):
            evidence = status_text
        value, explicit = grade(found, status_text, row[0].startswith("SM-"))
        result.append({
            "id": row[0], "file": path.name, "line": index + 1,
            "title": one_line(row[1]) if len(row) > 1 else row[0],
            "grade": value, "explicit": explicit, "evidence": one_line(evidence),
            "has_status": bool(status_cells),
        })
    return result


def escape_cell(text):
    return re.sub(r"(?<!\\)\|", r"\\|", text)


def abbreviate(text, limit):
    text = escape_cell(one_line(text))
    if len(text) <= limit:
        return text
    cut = text[:limit]
    for separator in ("; ", ", ", " "):
        position = cut.rfind(separator)
        if position > limit // 2:
            cut = cut[:position]
            break
    if cut.count("`") % 2:
        cut = cut[:cut.rfind("`")]
    if cut.count('"') % 2:
        cut = cut[:cut.rfind('"')]
    cut = cut.rstrip(" ;,(/")
    if cut.endswith("\\"):
        cut = cut[:-1]
    return cut + " ..."


def has_checker(text):
    """A checker is real text: not blank and not a none/n/a/missing/tbd placeholder."""
    return bool(text.strip()) and not re.match(r"\W*(?:none|n/a|missing|tbd)\b", text, re.I)


def derive(root):
    files = sorted(path for path in root.glob("*.md") if re.match(r"(?:0[1-9]|10)-", path.name))
    owners, problems = {}, []
    headings = collections.defaultdict(list)
    rows = collections.defaultdict(list)
    for path in files:
        for entry in heading_entries(path):
            headings[entry["id"]].append(entry)
        for entry in table_rows(path):
            rows[entry["id"]].append(entry)
    for key, entries in headings.items():
        if len(entries) > 1:
            problems.append(f"{key}: several headings {[e['file'] for e in entries]}")
        owners[key] = entries[0]
    for key, entries in rows.items():
        if key in owners:
            owner = owners[key]
            for summary in entries:
                if summary["has_status"] and summary["grade"] != owner["grade"]:
                    problems.append(
                        f"{key}: summary row {summary['file']}:{summary['line']} says {summary['grade']}, "
                        f"entry {owner['file']}:{owner['line']} says {owner['grade']}"
                    )
            continue
        if len(entries) > 1:
            problems.append(f"{key}: several owning rows {[(e['file'], e['line']) for e in entries]}")
        owners[key] = entries[0]
    if not owners:
        problems.append(f"no owned IDs found in {root}")
    for key, owner in owners.items():
        where = f"{owner['file']}:{owner['line']}"
        if not owner["explicit"]:
            problems.append(f"{key}: no status recorded at {where}")
        critical = owner.get("critical")
        if key.startswith("KC-") and critical is not None:
            if not critical.strip():
                problems.append(f"{key}: no Critical field at {where}")
            elif not re.match(r"\W*(?:yes|no)\b", critical, re.I):
                problems.append(f"{key}: Critical field is neither yes nor no at {where}")
        if re.match(r"\W*yes\b", critical or "", re.I) and not has_checker(owner.get("checkers", "")):
            problems.append(f"{key}: critical entry has no checker at {where}")
    for path in sorted(path for path in root.glob("*.md") if path.name != "11-traceability.md"):
        for number, line in enumerate(path.read_text().splitlines(), 1):
            for match in ID_ANY.finditer(line):
                if match[0] not in owners:
                    problems.append(f"{match[0]}: unknown ID at {path.name}:{number}")
    return owners, problems


def check_references(owners, root):
    """Resolve the Go checks named in the evidence, Checkers and Fails on fields against this checkout."""
    top = subprocess.run(["git", "-C", str(root), "rev-parse", "--show-toplevel"], capture_output=True, text=True)
    if top.returncode:
        return {"status": "not checked: the verification set is not in a git checkout"}
    repo = pathlib.Path(top.stdout.strip())
    files = subprocess.run(
        ["git", "-C", str(repo), "ls-files", "*_test.go"], capture_output=True, text=True, check=True
    ).stdout.splitlines()
    defined = set()
    for name in files:
        defined.update(GO_DEFINITION.findall((repo / name).read_text(errors="replace")))
    paths = set(files)
    basenames = {pathlib.PurePosixPath(name).name for name in files}
    present, missing = 0, {}
    for key in sorted(owners):
        evidence = owners[key]["evidence"]
        checkers = owners[key].get("checkers", "")
        fails_on = owners[key].get("fails_on", "")
        text = " ".join(part for part in (evidence, checkers, fails_on) if part)
        for check in sorted(set(GO_CHECK.findall(text)) | set(GO_TEST_FILE.findall(text))):
            if check.endswith("_test.go"):
                found = check in paths if "/" in check else check in basenames
            else:
                found = check in defined
            if found:
                present += 1
            else:
                missing.setdefault(key, []).append(check)
    return {
        "status": (
            "missing: Go checks named in an evidence field (Tests/Rehearsed) or in the Checkers or Fails on "
            "field but not on this branch; in the reservations set the entry's pinned citations hold the evidence"
        ),
        "present": present,
        "missing": sum(len(checks) for checks in missing.values()),
        "missing_by_id": missing,
    }


def render(owners):
    summary = collections.defaultdict(collections.Counter)
    for owner in owners.values():
        summary[owner["file"][:2]][owner["grade"]] += 1
    total = collections.Counter(owner["grade"] for owner in owners.values())
    out = [
        "# Reservations - traceability matrix",
        "",
        "This matrix maps every canonical review ID to its owning entry, for the prefixes SM, PAR, INV, US, EC, FM, DR, TM, XR and AR.",
        "The VF- findings are owned by [finding adjudication](12-finding-adjudication.md) and have no row here.",
        "Each ID appears once. A heading entry owns its ID; a summary-table row for the same ID must record the same status.",
        "Rows are derived from the owning entries: the title, the most cautious recorded status, and the test or rehearsal field.",
        "When a table has no test column, the evidence cell uses the status cell if that cell names a test.",
        "`model` denotes a descriptive row; `pointer` refers to other items and adds no coverage.",
        "No status in this matrix asserts that the named tests ran during this documentation task.",
        "",
        "## Coverage summary",
        "",
        "| Chapter | Items | " + " | ".join(COLUMNS) + " |",
        "|---|---|" + "---|" * len(COLUMNS),
    ]
    for prefix, name in CHAPTERS:
        counts = summary[prefix]
        out.append(f"| {name} | {sum(counts.values())} | " + " | ".join(str(counts[c]) for c in COLUMNS) + " |")
    out.append(f"| **Total** | {len(owners)} | " + " | ".join(str(total[c]) for c in COLUMNS) + " |")
    out += [
        "",
        "## Review queue rule",
        "",
        "Start with the `violated`, `unknown`, `design-only` and `holds-untested` rows below.",
        "Read [finding adjudication](12-finding-adjudication.md) before treating a draft observation as a defect.",
        "[Approved design deltas](13-approved-design-deltas.md) separate the D31 and D28 target design from the inspected implementation.",
        "`none recorded` means that the owning entry has no test or rehearsal field. It does not mean that the repository has no related tests.",
        "",
        "## Item matrix",
        "",
    ]
    for prefix, name in CHAPTERS:
        out += [f"### {name}", "", "| ID and owning file | Item | Status | Test / rehearsal reference |", "|---|---|---|---|"]
        for key in sorted(owners):
            owner = owners[key]
            if owner["file"][:2] != prefix:
                continue
            out.append(
                f"| [{key}]({owner['file']}) | {escape_cell(owner['title'])} | {owner['grade']} | "
                f"{abbreviate(owner['evidence'], 160) or 'none recorded'} |"
            )
        out.append("")
    out += [
        "## Maintenance",
        "",
        "Change the owning entry, then regenerate this file with `tools/traceability.py`. Do not edit its rows by hand.",
        "A reviewer can check any row against its linked entry: same title, same most cautious status, same test field.",
        "Keep immutable implementation pins in the owning chapter. Do not replace a historical citation with a moving branch name.",
        "Source inspection, a named behavior test and an exercised runtime scenario are different evidence levels.",
        "",
    ]
    return "\n".join(out)


def main():
    root = pathlib.Path(sys.argv[1]).resolve()
    owners, problems = derive(root)
    text = render(owners)
    if "--check" not in sys.argv:
        sys.stdout.write(text)
        return 0
    matrix = root / "11-traceability.md"
    if matrix.exists():
        current = matrix.read_text()
        matches = current == text
        changed = [] if matches else [
            line for line in difflib.unified_diff(current.splitlines(), text.splitlines(), "committed", "derived", lineterm="", n=0)
            if line.startswith(("+", "-")) and line not in ("--- committed", "+++ derived")
        ]
        matrix_report = {
            "matrix_matches_derivation": matches,
            "stale_matrix_lines": sum(line.startswith(("+|", "-|")) for line in changed),
            "changed_examples": changed[:24] or ([] if matches else ["(only line endings or the final newline differ)"]),
        }
    else:
        matrix_report = {"matrix_matches_derivation": None, "stale_matrix_lines": 0, "changed_examples": []}
    report = {
        "owned_ids": len(owners),
        "status_counts": collections.Counter(owner["grade"] for owner in owners.values()),
        "problems": problems,
        "check_references": check_references(owners, root),
    }
    report.update(matrix_report)
    print(json.dumps(report, indent=2))
    return 1 if problems or matrix_report["matrix_matches_derivation"] is False else 0


if __name__ == "__main__":
    sys.exit(main())
