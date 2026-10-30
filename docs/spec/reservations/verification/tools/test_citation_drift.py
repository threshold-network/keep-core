"""Behavior tests for citation_drift.py.

Each test builds a temporary tbtc-v2 repository whose test file shifts between
two commits, a temporary spec directory with a pinned citation, and runs
`python3 citation_drift.py` as a CLI.

The four cases the tool has to get right:
  - a re-pinned citation whose number moved but whose title did not is a
    correct re-pin and must pass;
  - a citation whose number stayed but now lands on a different test is drift
    and must fail;
  - a title that prettier wrapped onto the next line must resolve;
  - the README paragraph must describe the flow the tool implements.

Run from this folder: python3 -m unittest test_citation_drift
Requires git only.
"""
import os
import pathlib
import re
import subprocess
import sys
import tempfile
import unittest

TOOLS = pathlib.Path(__file__).resolve().parent
DRIFT = TOOLS / "citation_drift.py"
README = TOOLS.parent / "README.md"

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

# The same file, but the cited test moves down by one line and a different
# test takes its place, so the citation keeps its number and changes its title.
SAME_NUMBER_OTHER_TITLE = """\
describe("caps", () => {
  it("rejects a per-wallet position cap of zero", async () => {
    expect(1).to.equal(1)
  })
  it("rejects a term below the protocol floor", async () => {
    expect(2).to.equal(2)
  })
})
"""

# prettier wrapped the call, so the title is on the line after the it(
WRAPPED = """\
describe("caps", () => {
  it(
    "rejects a term below the protocol floor",
    async () => {
      expect(1).to.equal(1)
    }
  )
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

    def snapshot(self, revision):
        snapshot = self.root / f"base-{revision}.json"
        written = run(self.spec, self.repo, revision, "--write", str(snapshot))
        self.assertEqual(written.returncode, 0, written.stderr)
        return snapshot

    def test_a_repin_that_moves_the_number_but_keeps_the_title_passes(self):
        """A correct re-pin renumbers the citation; that is not drift.

        The re-pin rewrites the spec's citation too, so the cited test is the
        same one at a new number in the new file. The citation's place in the
        spec does not move, so the two snapshots have the same key and the
        titles match.
        """
        before = make_repo(self.repo, {TEST_FILE: BEFORE}, "before")
        after = make_repo(self.repo, {TEST_FILE: AFTER}, "after")
        make_spec(self.spec, f"tbtc-v2@{before}:{TEST_FILE}:5")

        snapshot = self.snapshot(before)
        # The re-pin rewrites the citation: the same test, now at line 8.
        make_spec(self.spec, f"tbtc-v2@{after}:{TEST_FILE}:8")
        result = run(self.spec, self.repo, after, "--baseline", str(snapshot))
        self.assertEqual(result.returncode, 0, result.stdout)
        self.assertIn("drifted: 0", result.stdout)
        self.assertIn("RESULT: PASS", result.stdout)

    def test_a_citation_that_lands_on_another_test_fails(self):
        """The same number naming a different test is drift."""
        before = make_repo(self.repo, {TEST_FILE: BEFORE}, "before")
        after = make_repo(self.repo, {TEST_FILE: SAME_NUMBER_OTHER_TITLE}, "after")
        make_spec(self.spec, f"tbtc-v2@{before}:{TEST_FILE}:5")

        snapshot = self.snapshot(before)
        result = run(self.spec, self.repo, after, "--baseline", str(snapshot))
        self.assertEqual(result.returncode, 1, result.stdout)
        self.assertIn("drifted: 1", result.stdout)
        self.assertIn("rejects a term below the protocol floor", result.stdout)
        self.assertIn("rejects a per-wallet position cap of zero", result.stdout)
        self.assertIn("RESULT: FAIL", result.stdout)

    def test_a_wrapped_title_resolves(self):
        """prettier wraps it( so the title is on the next line; it still resolves."""
        revision = make_repo(self.repo, {TEST_FILE: WRAPPED}, "wrapped")
        make_spec(self.spec, f"tbtc-v2@{revision}:{TEST_FILE}:2")

        result = run(self.spec, self.repo, revision)
        self.assertIn("resolvable=1", result.stdout)
        self.assertNotIn("no-title", result.stdout)

    def test_the_key_does_not_depend_on_the_source_number(self):
        """A citation is identified by its place in the spec, not by its target.

        The re-pin changes the number, so a key built from the number would make
        every moved citation look new and the documented flow could never pass.
        """
        before = make_repo(self.repo, {TEST_FILE: BEFORE}, "before")
        after = make_repo(self.repo, {TEST_FILE: AFTER}, "after")
        make_spec(self.spec, f"tbtc-v2@{before}:{TEST_FILE}:5")

        first = self.snapshot(before)
        second = self.snapshot(after)
        import json
        keys_before = set(json.loads(first.read_text()))
        keys_after = set(json.loads(second.read_text()))
        self.assertEqual(keys_before, keys_after)

    def test_a_path_change_is_reported_separately(self):
        """A citation that moves to another file is deliberate, not drift."""
        before = make_repo(self.repo, {TEST_FILE: BEFORE}, "before")
        after = make_repo(
            self.repo,
            {TEST_FILE: BEFORE, "solidity/test/bridge/Other.test.ts": BEFORE},
            "after",
        )
        make_spec(self.spec, f"tbtc-v2@{before}:{TEST_FILE}:5")

        snapshot = self.snapshot(before)
        result = run(self.spec, self.repo, after, "--baseline", str(snapshot))
        self.assertEqual(result.returncode, 0, result.stdout)
        self.assertIn("path changed: 0", result.stdout)

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

    def test_the_readme_describes_the_flow_the_tool_implements(self):
        """The README must name the snapshot and baseline steps, in that order."""
        text = README.read_text(encoding="utf-8")
        paragraph = [
            line for line in text.splitlines()
            if "citation_drift.py" in line and "--baseline" in line
        ]
        self.assertTrue(paragraph, "the README has no citation drift paragraph")
        line = paragraph[0]
        self.assertIn("--write", line, "the README does not mention writing a snapshot")
        self.assertIn("--baseline", line, "the README does not mention the baseline compare")
        self.assertLess(
            line.index("--write"), line.index("--baseline"),
            "the README must describe writing the snapshot before comparing",
        )
        self.assertIn("drifted: 0", line, "the README does not state the required result")
        self.assertTrue(
            re.search(r"numbered by|number|the source number", line),
            "the README does not explain that the number may move",
        )


if __name__ == "__main__":
    unittest.main()
