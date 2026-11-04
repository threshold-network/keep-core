"""Behavior tests for smoke.py.

Each test builds a temporary verification directory and temporary git
repositories, runs `python3 smoke.py` as a CLI, and checks the exit code and the
JSON report. Run from this folder: python3 -m unittest test_smoke
Requires git and markdown-it-py.
"""
import json
import pathlib
import subprocess
import sys
import tempfile
import unittest

SMOKE = pathlib.Path(__file__).resolve().parent / "smoke.py"


def git(path, *args):
    result = subprocess.run(["git", "-C", str(path), *args], capture_output=True, text=True, check=True)
    return result.stdout.strip()


def make_repo(path, files):
    path.mkdir(parents=True)
    git(path, "init", "-q")
    for name, content in files.items():
        target = path / name
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_text(content)
        git(path, "add", name)
    return git(path, "write-tree")


class SmokeTest(unittest.TestCase):
    def setUp(self):
        tmp = tempfile.TemporaryDirectory()
        self.addCleanup(tmp.cleanup)
        self.work = pathlib.Path(tmp.name)
        self.tbtc = self.work / "tbtc-v2"
        self.keep = self.work / "keep-core"
        self.tbtc_rev = make_repo(self.tbtc, {
            "contracts/A.sol": 'require(a, "Cap exceeded");\nline two \u2014 dash\nsplit\nacross lines\n',
            "contracts/B.sol": 'require(b, "Other message");\n',
        })
        self.keep_rev = make_repo(self.keep, {"client/main.go": 'log("stub value")\n'})
        self.docs = self.work / "verification"
        self.docs.mkdir()
        (self.docs / "11-traceability.md").write_text("# Traceability\n")

    def tc(self, path, line="1", rev=None):
        return f"`tbtc-v2@{rev or self.tbtc_rev}:{path}:{line}`"

    def kc(self, path, line="1"):
        return f"`keep-core@{self.keep_rev}:{path}:{line}`"

    def run_smoke(self, doc, tbtc=None, keep=None, name="02-invariants.md"):
        (self.docs / name).write_text(doc)
        result = subprocess.run(
            [sys.executable, str(SMOKE), str(self.docs), str(tbtc or self.tbtc), str(keep or self.keep)],
            capture_output=True, text=True,
        )
        return result.returncode, json.loads(result.stdout), result.stderr

    def test_exact_quote_before_and_after_citation(self):
        code, report, _ = self.run_smoke(
            f'Reverts "Cap exceeded" {self.tc("contracts/A.sol")}.\n\n'
            f'{self.tc("contracts/B.sol")} "Other message"\n'
        )
        self.assertEqual(code, 0, report)
        self.assertEqual((report["quotes_checked"], report["quotes_matched"], report["quote_warnings"]), (2, 2, 0))
        self.assertEqual(report["explicit_pinned_citations_checked"], 2)
        self.assertEqual(report["citations_skipped"], 0)

    def test_mismatch_warns_and_exits_zero(self):
        code, report, _ = self.run_smoke(f'Says "Cap raised" {self.tc("contracts/A.sol")}.\n')
        self.assertEqual(code, 0, report)
        self.assertEqual(report["errors"], [])
        self.assertEqual(report["quote_warnings"], 1)
        self.assertEqual(report["quotes_matched"], 0)
        self.assertIn("Cap raised", report["warnings"][0])

    def test_broken_source_still_fails_beside_warning(self):
        code, report, _ = self.run_smoke(
            f'Says "Cap raised" {self.tc("contracts/A.sol")}.\n\n'
            f'Gone "x" {self.tc("contracts/Missing.sol")}.\n\n'
            f'Far {self.tc("contracts/A.sol", "99")}.\n'
        )
        self.assertEqual(code, 1)
        self.assertEqual(report["quote_warnings"], 1)
        self.assertEqual(len([e for e in report["errors"] if "absent source" in e]), 1)
        self.assertEqual(len([e for e in report["errors"] if "exceeds file" in e]), 1)
        self.assertEqual(report["quotes_unverifiable"], 1)

    def test_pairs_in_one_paragraph_keep_their_own_file(self):
        doc = (
            f'"Cap exceeded" {self.tc("contracts/A.sol")} and "Other message" {self.tc("contracts/B.sol")}; '
            f'"Other message" {self.tc("contracts/B.sol")}, "Cap exceeded" {self.tc("contracts/A.sol")}\n'
        )
        code, report, _ = self.run_smoke(doc)
        self.assertEqual(code, 0, report)
        self.assertEqual((report["quotes_checked"], report["quote_warnings"]), (4, 0))

    def test_swapped_pairs_warn_per_pair_and_ambiguous_middle_matches(self):
        a, b = self.tc("contracts/A.sol"), self.tc("contracts/B.sol")
        code, report, _ = self.run_smoke(f'"Other message" {a} then "Cap exceeded" {b}\n')
        self.assertEqual(code, 0, report)
        self.assertEqual((report["quotes_checked"], report["quote_warnings"]), (2, 2))
        code, report, _ = self.run_smoke(f'"Other message" {a}, "Cap exceeded" {b}\n')
        self.assertEqual((report["quotes_checked"], report["quote_warnings"]), (2, 2))
        code, report, _ = self.run_smoke(f'{a} "Cap exceeded" {b}\n')
        self.assertEqual((report["quotes_checked"], report["quote_warnings"]), (1, 0))

    def test_unrelated_quotes_are_not_associated(self):
        doc = (
            f'The word "holds" appears; see {self.tc("contracts/A.sol")}. A later "Nothing" sentence.\n\n'
            f'| col | col |\n|---|---|\n| "Cap exceeded" | {self.tc("contracts/A.sol")} |\n'
        )
        code, report, _ = self.run_smoke(doc)
        self.assertEqual(code, 0, report)
        self.assertEqual((report["quotes_checked"], report["quote_warnings"], report["quotes_unassociated"]), (0, 0, 3))

    def test_alternates_and_normalization(self):
        doc = (
            f'"Gone" or "Cap exceeded" {self.tc("contracts/A.sol")}\n\n'
            f'{self.tc("contracts/A.sol", "1")}, {self.tc("contracts/B.sol")} "Other message"\n\n'
            f'"line two - dash" {self.tc("contracts/A.sol", "2")}\n\n'
            f'"split across" {self.tc("contracts/A.sol", "3-4")}\n\n'
            f'"line two ... dash" {self.tc("contracts/A.sol", "2")}\n'
        )
        code, report, _ = self.run_smoke(doc)
        self.assertEqual(code, 0, report)
        self.assertEqual((report["quotes_checked"], report["quote_warnings"]), (6, 1), report)

    def test_multiline_paragraph_is_one_unit(self):
        doc = f'"Cap\nexceeded" is the revert\n{self.tc("contracts/A.sol")}\n'
        code, report, _ = self.run_smoke(doc)
        self.assertEqual(report["quotes_unassociated"], 1)
        doc = f'"Cap\nexceeded"\n{self.tc("contracts/A.sol")}\n'
        code, report, _ = self.run_smoke(doc)
        self.assertEqual((report["quotes_checked"], report["quote_warnings"]), (1, 0))

    def test_partial_mode_never_touches_skipped_repo(self):
        missing = self.work / "missing-tbtc"
        doc = (
            f'"Cap exceeded" {self.tc("contracts/A.sol")} and {self.tc("contracts/B.sol")} and\n\n'
            f'"stub value" {self.kc("client/main.go")}\n\n'
            f'"nope" {self.kc("client/main.go")}\n'
        )
        code, report, stderr = self.run_smoke(doc, tbtc="-")
        self.assertEqual(code, 0, report)
        self.assertEqual((report["explicit_pinned_citations_checked"], report["citations_skipped"]), (2, 2))
        self.assertEqual(report["skipped_repositories"], ["tbtc-v2"])
        self.assertEqual((report["quotes_checked"], report["quotes_matched"], report["quote_warnings"]), (2, 1, 1))
        self.assertEqual(report["quotes_skipped"], 1)
        code_missing, report_missing, _ = self.run_smoke(doc, tbtc=str(missing))
        self.assertEqual(code_missing, 1)
        self.assertEqual(report_missing["citations_skipped"], 0)
        self.assertFalse(missing.exists())

    def test_partial_mode_still_checks_keep_core(self):
        doc = f'Broken {self.kc("client/missing.go")} and {self.tc("contracts/Missing.sol")}\n'
        code, report, _ = self.run_smoke(doc, tbtc="-")
        self.assertEqual(code, 1)
        self.assertEqual(len(report["errors"]), 1)
        self.assertIn("client/missing.go", report["errors"][0])
        self.assertEqual(report["citations_skipped"], 1)


if __name__ == "__main__":
    unittest.main()
