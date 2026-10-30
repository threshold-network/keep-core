"""Behavior tests for citation_drift.py.

Each test builds a temporary tbtc-v2 repository whose test file shifts between
two commits, a temporary spec directory with a pinned citation, and runs
`python3 citation_drift.py` as a CLI. The check under test is that a citation
which no longer names the test it named is reported as drift.

Run from this folder: python3 -m unittest test_citation_drift
Requires git only.
"""
import os
import pathlib
import subprocess
import sys
import tempfile
import unittest

TOOLS = pathlib.Path(__file__).resolve().parent
DRIFT = TOOLS / "citation_drift.py"

IDENTITY = {
    "GIT_AUTHOR_NAME": "Citation Drift Test",
    "GIT_AUTHOR_EMAIL": "drift@example.invalid",
    "GIT_COMMITTER_NAME": "Citation Drift Test",
    "GIT_COMMITTER_EMAIL": "drift@example.invalid",
}

TEST_FILE = "solidity/test/bridge/Bridge.ReservationCaps.test.ts"

BEFORE = """\
describe("caps", () => {
  it("rejects a term below the protocol floor", async () => {
    expect(1).to.equal(1)
  })
  it("rejects a per-wallet position cap of zero", async () => {
    expect(2).to.equal(2)
  })
})
"""

# Three lines are inserted above, so the citation at line 5 no longer lands on
# the second test: the same number now names the first one.
AFTER = """\
// an upstream edit
// that shifts the file
// by three lines
describe("caps", () => {
  it("rejects a term below the protocol floor", async () => {
    expect(1).to.equal(1)
  })
  it("rejects a per-wallet position cap of zero", async () => {
    expect(2).to.equal(2)
  })
})
"""


def git(path, *args):
    result = subprocess.run(
        ["git", "-C", str(path), *args], capture_output=True, text=True, check=True,
        env=dict(os.environ, **IDENTITY),
    )
    return result.stdout.strip()


def make_repo(path, files, message):
    path.mkdir(parents=True, exist_ok=True)
    if not (path / ".git").exists():
        git(path, "init", "-q")
    for name, text in files.items():
        target = path / name
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_text(text, encoding="utf-8")
    git(path, "add", "-A")
    git(path, "commit", "-q", "-m", message)
    return git(path, "rev-parse", "HEAD")


def make_spec(path, citation):
    """A spec directory with one pinned citation into the test file."""
    path.mkdir(parents=True, exist_ok=True)
    (path / "row.md").write_text(
        f'| PAR-3 | term bounds | `holds` | Direct setter test: `{citation}` |\n',
        encoding="utf-8",
    )
    return path


def run(spec, repo, revision, *extra):
    return subprocess.run(
        [sys.executable, str(DRIFT), str(spec), str(repo), revision, *extra],
        capture_output=True, text=True,
    )


class CitationDriftTest(unittest.TestCase):
    def setUp(self):
        self.root = pathlib.Path(tempfile.mkdtemp())
        self.repo = self.root / "tbtc-v2"
        self.spec = self.root / "spec"

    def test_a_shifted_file_is_reported_as_drift(self):
        """A citation whose line now lands on another test is drift, not a pass."""
        before = make_repo(self.repo, {TEST_FILE: BEFORE}, "before")
        after = make_repo(self.repo, {TEST_FILE: AFTER}, "after")
        make_spec(self.spec, f"tbtc-v2@{before}:{TEST_FILE}:5")

        snapshot = self.root / "base.json"
        written = run(self.spec, self.repo, before, "--write", str(snapshot))
        self.assertEqual(written.returncode, 0, written.stderr)

        result = run(self.spec, self.repo, after, "--baseline", str(snapshot))
        self.assertEqual(result.returncode, 1, result.stdout)
        self.assertIn("drifted: 1", result.stdout)
        self.assertIn("rejects a term below the protocol floor", result.stdout)
        self.assertIn("rejects a per-wallet position cap of zero", result.stdout)
        self.assertIn("RESULT: FAIL", result.stdout)

    def test_an_unshifted_file_reports_no_drift(self):
        """The same revision compared with itself is clean."""
        revision = make_repo(self.repo, {TEST_FILE: BEFORE}, "before")
        make_spec(self.spec, f"tbtc-v2@{revision}:{TEST_FILE}:5")

        snapshot = self.root / "base.json"
        run(self.spec, self.repo, revision, "--write", str(snapshot))
        result = run(self.spec, self.repo, revision, "--baseline", str(snapshot))
        self.assertEqual(result.returncode, 0, result.stdout)
        self.assertIn("drifted: 0", result.stdout)
        self.assertIn("RESULT: PASS", result.stdout)

    def test_a_bare_citation_is_not_checked(self):
        """A citation without a revision is the pin rule's business, not this tool's."""
        revision = make_repo(self.repo, {TEST_FILE: BEFORE}, "before")
        make_spec(self.spec, f"tbtc-v2:{TEST_FILE}:5")

        snapshot = self.root / "base.json"
        written = run(self.spec, self.repo, revision, "--write", str(snapshot))
        self.assertEqual(written.returncode, 0, written.stderr)
        self.assertIn("0 citations", written.stdout)

    def test_an_absent_file_is_reported(self):
        """A pinned path that does not exist at the revision is reported, not ignored."""
        revision = make_repo(self.repo, {TEST_FILE: BEFORE}, "before")
        make_spec(self.spec, f"tbtc-v2@{revision}:solidity/test/bridge/Missing.test.ts:3")

        result = run(self.spec, self.repo, revision)
        self.assertIn("absent", result.stdout)


if __name__ == "__main__":
    unittest.main()
