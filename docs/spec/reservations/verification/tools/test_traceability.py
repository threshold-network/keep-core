"""Behavior tests for traceability.py --check.

Each test builds a temporary git checkout with a small verification set, runs
`python3 traceability.py <set> --check` as a CLI, and checks the exit code and
the JSON report. Run from this folder: python3 -m unittest test_traceability
Requires git.
"""
import json
import pathlib
import subprocess
import sys
import tempfile
import unittest

TRACEABILITY = pathlib.Path(__file__).resolve().parent / "traceability.py"

ENTRY = "## {key} {title}\n\n- **Test.** {test}\n- **Status.** holds.\n\n"


class TraceabilityCheckTest(unittest.TestCase):
    def setUp(self):
        tmp = tempfile.TemporaryDirectory()
        self.addCleanup(tmp.cleanup)
        self.repo = pathlib.Path(tmp.name)
        subprocess.run(["git", "-C", str(self.repo), "init", "-q"], check=True)
        (self.repo / "pkg").mkdir()
        (self.repo / "pkg/present_test.go").write_text("package pkg\n\nfunc TestPresent(t *testing.T) {}\n")
        subprocess.run(["git", "-C", str(self.repo), "add", "pkg/present_test.go"], check=True)
        self.docs = self.repo / "verification"
        self.docs.mkdir()

    def check(self, chapter, extra=None):
        (self.docs / "02-invariants.md").write_text("# Invariants\n\n" + chapter)
        for name, text in (extra or {}).items():
            (self.docs / name).write_text(text)
        derived = subprocess.run(
            [sys.executable, str(TRACEABILITY), str(self.docs)], capture_output=True, text=True, check=True
        ).stdout
        (self.docs / "11-traceability.md").write_text(derived)
        result = subprocess.run(
            [sys.executable, str(TRACEABILITY), str(self.docs), "--check"], capture_output=True, text=True
        )
        return result.returncode, json.loads(result.stdout)

    def test_missing_check_is_reported_and_does_not_fail(self):
        code, report = self.check(
            ENTRY.format(key="INV-1", title="Present", test="`TestPresent` in `pkg/present_test.go`")
            + ENTRY.format(key="INV-2", title="Absent", test="`TestAbsent` in `pkg/absent_test.go`")
        )
        self.assertEqual(code, 0)
        self.assertEqual(report["problems"], [])
        references = report["check_references"]
        self.assertEqual(references["present"], 2)
        self.assertEqual(references["missing_by_id"], {"INV-2": ["TestAbsent", "pkg/absent_test.go"]})

    def test_unknown_id_fails(self):
        code, report = self.check(
            ENTRY.format(key="INV-1", title="Refers to INV-9", test="none"),
            {"12-finding-adjudication.md": "# Findings\n\nSee INV-8.\n"},
        )
        self.assertEqual(code, 1)
        self.assertEqual(
            report["problems"],
            ["INV-9: unknown ID at 02-invariants.md:3", "INV-8: unknown ID at 12-finding-adjudication.md:3"],
        )

    def test_duplicate_heading_fails(self):
        code, report = self.check(ENTRY.format(key="INV-1", title="One", test="none") * 2)
        self.assertEqual(code, 1)
        self.assertIn("INV-1: several headings ['02-invariants.md', '02-invariants.md']", report["problems"])

    def test_entry_without_status_fails(self):
        code, report = self.check("## INV-1 No status\n\n- **Test.** none\n\n")
        self.assertEqual(code, 1)
        self.assertIn("INV-1: no status recorded at 02-invariants.md:3", report["problems"])

    def test_stale_matrix_fails(self):
        (self.docs / "02-invariants.md").write_text("# Invariants\n\n" + ENTRY.format(key="INV-1", title="T", test="none"))
        (self.docs / "11-traceability.md").write_text("# Stale\n")
        result = subprocess.run(
            [sys.executable, str(TRACEABILITY), str(self.docs), "--check"], capture_output=True, text=True
        )
        self.assertEqual(result.returncode, 1)
        self.assertFalse(json.loads(result.stdout)["matrix_matches_derivation"])

    def check_only(self, chapter):
        (self.docs / "02-invariants.md").write_text("# Invariants\n\n" + chapter)
        result = subprocess.run(
            [sys.executable, str(TRACEABILITY), str(self.docs), "--check"], capture_output=True, text=True
        )
        return result.returncode, json.loads(result.stdout)

    def test_critical_entry_without_checker_fails(self):
        entry = (
            "## INV-9 Critical\n"
            "\n"
            "- **Test.** none\n"
            "- **Critical.** yes\n"
            "- **Status.** holds.\n"
        )
        code, report = self.check_only(entry)
        self.assertEqual(code, 1)
        self.assertIn("INV-9: critical entry has no checker at 02-invariants.md:3", report["problems"])

    def test_critical_entry_with_checker_passes_without_matrix(self):
        entry = (
            "## INV-9 Critical\n"
            "\n"
            "- **Test.** none\n"
            "- **Checkers.**\n"
            "  - proof, `solidity/ecdsa/test/halmos/keep-core/KeepApproval.t.sol` `check_publicApprovalCommitmentOrRollback`, passing.\n"
            "- **Critical.** yes\n"
            "- **Exposure.** custodied balance of one wallet, about one BTC.\n"
            "- **Fails on.**\n"
            "- **Status.** holds.\n"
        )
        code, report = self.check_only(entry)
        self.assertEqual(code, 0)
        self.assertEqual(report["problems"], [])
        self.assertIsNone(report["matrix_matches_derivation"])

    def test_non_critical_entry_without_checker_passes(self):
        entry = (
            "## INV-9 Not critical\n"
            "\n"
            "- **Test.** none\n"
            "- **Critical.** no\n"
            "- **Status.** holds.\n"
        )
        code, report = self.check_only(entry)
        self.assertEqual(code, 0)
        self.assertEqual(report["problems"], [])

    def test_kc_id_is_owned(self):
        entry = (
            "## KC-101 DKG result\n"
            "\n"
            "- **Test.** none\n"
            "- **Checkers.** proof, `check_publicApprovalCommitmentOrRollback`, passing.\n"
            "- **Critical.** yes\n"
            "- **Status.** holds.\n"
        )
        code, report = self.check_only(entry)
        self.assertEqual(code, 0)
        self.assertEqual(report["owned_ids"], 1)

    def test_critical_value_must_lead_with_yes(self):
        entry = (
            "## INV-9 Not critical\n"
            "\n"
            "- **Test.** none\n"
            "- **Critical.** no - reviewers say yes\n"
            "- **Status.** holds.\n"
        )
        code, report = self.check_only(entry)
        self.assertEqual(code, 0)
        self.assertEqual(report["problems"], [])

    def test_placeholder_checkers_do_not_satisfy_critical(self):
        for placeholder in ("none", "None.", "missing", "n/a", "TBD", ""):
            with self.subTest(placeholder=placeholder):
                entry = (
                    "## INV-9 Critical\n"
                    "\n"
                    f"- **Checkers.** {placeholder}\n"
                    "- **Critical.** yes\n"
                    "- **Status.** holds.\n"
                )
                code, report = self.check_only(entry)
                self.assertEqual(code, 1)
                self.assertIn("INV-9: critical entry has no checker at 02-invariants.md:3", report["problems"])

    def test_kc_entry_needs_critical_yes_or_no(self):
        base = "## KC-101 Entry\n\n- **Test.** none\n{critical}- **Status.** holds.\n"
        code, report = self.check_only(base.format(critical=""))
        self.assertEqual(code, 1)
        self.assertIn("KC-101: no Critical field at 02-invariants.md:3", report["problems"])
        code, report = self.check_only(base.format(critical="- **Critical.** maybe\n"))
        self.assertEqual(code, 1)
        self.assertIn("KC-101: Critical field is neither yes nor no at 02-invariants.md:3", report["problems"])

    def test_non_kc_entry_needs_no_critical_field(self):
        code, report = self.check_only("## INV-9 Plain\n\n- **Test.** none\n- **Status.** holds.\n")
        self.assertEqual(code, 0)
        self.assertEqual(report["problems"], [])

    def test_set_without_owned_ids_fails(self):
        code, report = self.check_only("")
        self.assertEqual(code, 1)
        self.assertEqual(report["owned_ids"], 0)
        self.assertEqual(len(report["problems"]), 1)
        self.assertTrue(report["problems"][0].startswith("no owned IDs found in "))

    def test_checkers_and_fails_on_checks_are_resolved(self):
        entry = (
            "## INV-9 Absent checks\n"
            "\n"
            "- **Test.** none\n"
            "- **Checkers.**\n"
            "  - property, `TestAbsent` in `pkg/absent_test.go`\n"
            "- **Fails on.** `TestAlsoAbsent`\n"
            "- **Status.** holds.\n"
        )
        code, report = self.check_only(entry)
        self.assertEqual(code, 0)
        self.assertEqual(
            report["check_references"]["missing_by_id"],
            {"INV-9": ["TestAbsent", "TestAlsoAbsent", "pkg/absent_test.go"]},
        )

    def test_status_after_empty_fails_on_is_parsed(self):
        code, report = self.check_only("## INV-9 Empty\n\n- **Test.** none\n- **Fails on.**\n- **Status.** holds.\n")
        self.assertEqual(code, 0)
        self.assertEqual(report["problems"], [])
        self.assertEqual(report["status_counts"], {"holds": 1})


if __name__ == "__main__":
    unittest.main()
