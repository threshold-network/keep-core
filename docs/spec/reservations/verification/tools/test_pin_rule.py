"""Behavior tests for pin_rule.py.

Each test builds temporary repositories with the pin ranges that pin-rule.json would
name, runs `python3 pin_rule.py` as a CLI against a temporary baseline, and checks the
exit code and the violation list. Run from this folder: python3 -m unittest test_pin_rule
Requires git only.
"""
import json
import os
import pathlib
import subprocess
import sys
import tempfile
import unittest

TOOLS = pathlib.Path(__file__).resolve().parent
PIN_RULE = TOOLS / "pin_rule.py"


IDENTITY = {
    "GIT_AUTHOR_NAME": "Pin Rule Test",
    "GIT_AUTHOR_EMAIL": "pin@example.invalid",
    "GIT_COMMITTER_NAME": "Pin Rule Test",
    "GIT_COMMITTER_EMAIL": "pin@example.invalid",
}


def git(path, *args):
    result = subprocess.run(
        ["git", "-C", str(path), *args], capture_output=True, text=True, check=True, env=dict(os.environ, **IDENTITY)
    )
    return result.stdout.strip()


def commit(path, message, parent=None):
    """Write the index as a commit without running hooks, as test_smoke does with trees."""
    args = ["commit-tree", git(path, "write-tree"), "-m", message]
    if parent:
        args += ["-p", parent]
    return git(path, *args)


def make_repo(path, files, parent=None, message="snapshot"):
    path.mkdir(parents=True, exist_ok=True)
    if not (path / ".git").exists():
        git(path, "init", "-q")
    write(path, files)
    git(path, "add", "-A")
    return commit(path, message, parent)


def write(path, files):
    for name, text in files.items():
        target = path / name
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_text(text, encoding="utf-8")


class PinRuleTest(unittest.TestCase):
    def setUp(self):
        self.root = pathlib.Path(tempfile.mkdtemp())
        self.keep = self.root / "keep-core"
        self.tbtc = self.root / "tbtc-v2"
        self.previous_tbtc = make_repo(self.tbtc, {
            "solidity/contracts/bridge/Reservation.sol": "line 1\nline 2\n",
            "solidity/contracts/bridge/Stable.sol": "steady\n",
        })
        self.previous_keep = make_repo(self.keep, {
            "pkg/tbtc/reservation.go": "package tbtc\n",
            "pkg/tbtc/steady.go": "package tbtc\n",
            "docs/spec/reservations/verification/README.md": "baseline\n",
        })
        write(self.tbtc, {"solidity/contracts/bridge/Reservation.sol": "line 1\nline 2\nline 3\n"})
        self.current_tbtc = make_repo(
            self.tbtc, {}, parent=self.previous_tbtc, message="changed"
        )
        write(self.keep, {"pkg/tbtc/reservation.go": "package tbtc\n\nfunc f() {}\n"})
        self.current_keep = make_repo(self.keep, {}, parent=self.previous_keep, message="changed")
        self.baseline = self.current_keep
        self.tools = self.keep / "docs/spec/reservations/verification/tools"
        self.tools.mkdir(parents=True)
        (self.tools / "pin_rule.py").write_text(PIN_RULE.read_text(encoding="utf-8"), encoding="utf-8")
        (self.tools / "repin.py").write_text((TOOLS / "repin.py").read_text(encoding="utf-8"), encoding="utf-8")
        (self.tools / "pin-rule.json").write_text(json.dumps({
            "baseline": self.baseline,
            "pins": {
                "tbtc-v2": {"previous": self.previous_tbtc, "current": self.current_tbtc},
                "keep-core": {"previous": self.previous_keep, "current": self.current_keep},
            },
            "paths": ["docs/spec/reservations"],
        }), encoding="utf-8")

    def run_rule(self):
        result = subprocess.run(
            [sys.executable, str(self.tools / "pin_rule.py"), str(self.keep), str(self.tbtc)],
            capture_output=True, text=True,
        )
        return result

    def add_line(self, text, name="docs/spec/reservations/verification/01-cases.md"):
        target = self.keep / name
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_text(text + "\n", encoding="utf-8")

    def test_bare_citation_into_a_changed_file_fails(self):
        self.add_line("Status `holds`: the bound is at `Reservation.sol:1`.")
        result = self.run_rule()
        self.assertEqual(result.returncode, 1, result.stdout + result.stderr)
        self.assertIn("solidity/contracts/bridge/Reservation.sol", result.stdout)
        self.assertIn("violations: 1", result.stdout)

    def test_bare_citation_into_an_unchanged_file_passes(self):
        self.add_line("Status `holds`: the same line is `Stable.sol:1`.")
        result = self.run_rule()
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("violations: 0", result.stdout)

    def test_explicit_pin_passes(self):
        self.add_line(f"Status `holds`: the bound is at `tbtc-v2@{self.current_tbtc}:solidity/contracts/bridge/Reservation.sol:1`.")
        result = self.run_rule()
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)

    def test_default_pin_prefix_into_a_changed_file_fails(self):
        self.add_line("Status `holds`: the reader is `keep-core:pkg/tbtc/reservation.go:1`.")
        result = self.run_rule()
        self.assertEqual(result.returncode, 1, result.stdout + result.stderr)
        self.assertIn("pkg/tbtc/reservation.go", result.stdout)

    def test_relative_citation_is_not_checked_on_its_own(self):
        self.add_line("Status `holds`: `Stable.sol:1` and the neighbour at `:2`.")
        result = self.run_rule()
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)

    def test_citation_that_predates_the_baseline_is_not_checked(self):
        older = make_repo(
            self.keep, {"docs/spec/reservations/verification/01-cases.md": "Old `Reservation.sol:1`.\n"},
            parent=self.current_keep, message="old text",
        )
        config = json.loads((self.tools / "pin-rule.json").read_text(encoding="utf-8"))
        config["baseline"] = older
        (self.tools / "pin-rule.json").write_text(json.dumps(config), encoding="utf-8")
        result = self.run_rule()
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("checked: 0 lines", result.stdout)


if __name__ == "__main__":
    unittest.main()