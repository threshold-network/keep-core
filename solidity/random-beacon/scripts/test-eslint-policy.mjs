// Lightweight policy test: lints small fixtures against the real
// eslint.config.mjs to confirm no-only-tests enforcement, ignore-pattern
// behavior, and that test/**/*.ts files are actually linted.
// Run with: node scripts/test-eslint-policy.mjs
//
// Fixtures are written as real, temporary files directly under this package's
// test/ and typechain/ directories (then deleted) so ESLint's `files`/`ignores`
// globs and the type-aware TS project both see a real, in-project path.
// The two test/-scoped checks deliberately reuse the SAME path, overwritten in
// place: typescript-eslint's project-based parser resolves its TS program
// once per process and never discovers a brand-new path added afterward
// (it fatal-errors "file was not found in any of the provided project(s)"),
// but it does correctly re-lint a path it has already resolved once its
// content changes - confirmed empirically before relying on it here.

/* eslint-disable import/no-extraneous-dependencies -- eslint is a legitimate devDependency used
   here to smoke-test this package's own lint config; it is not a runtime dependency. */
import { mkdirSync, rmSync, writeFileSync } from "node:fs"
import { dirname, join } from "node:path"
import { ESLint } from "eslint"

const pkgRoot = join(import.meta.dirname, "..")

let failures = 0

function assert(condition, message) {
  if (!condition) {
    failures += 1
    console.error(`FAIL: ${message}`)
  } else {
    console.log(`PASS: ${message}`)
  }
}

async function lintFixture(relativePath, code) {
  const fullPath = join(pkgRoot, relativePath)
  mkdirSync(dirname(fullPath), { recursive: true })
  writeFileSync(fullPath, code, "utf8")
  const eslint = new ESLint({ cwd: pkgRoot, errorOnUnmatchedPattern: false })
  const results = await eslint.lintFiles([fullPath])
  const messages = results[0]?.messages ?? []
  // Exclude ESLint's own synthetic "file ignored" notice (ruleId: null) so a
  // genuinely-ignored path with zero rule violations reads as zero messages.
  return messages.filter((m) => m.ruleId !== null)
}

const sharedTestFixture = "test/policy-fixture.ts"
const ignoredFixture = "typechain/policy-fixture-ignored.ts"

try {
  // Fixture (a): it.only in a test file should trigger no-only-tests
  const onlyMessages = await lintFixture(
    sharedTestFixture,
    'it.only("x", () => {})\n'
  )
  assert(
    onlyMessages.some((m) => m.ruleId === "no-only-tests/no-only-tests"),
    "it.only() in test/**/*.ts triggers no-only-tests/no-only-tests"
  )

  // Fixture (c): an ordinary test file should be linted (at least one rule evaluated).
  // Reuses sharedTestFixture's already-resolved path, overwritten with new content.
  const lintedMessages = await lintFixture(sharedTestFixture, "var y = 1\n")
  assert(
    lintedMessages.length > 0,
    "ordinary test/**/*.ts fixture is actually linted (not silently skipped)"
  )

  // Fixture (b): file matching an ignores glob (typechain/**) should have zero messages
  const ignoredMessages = await lintFixture(
    ignoredFixture,
    "const x: any = 1\n"
  )
  assert(
    ignoredMessages.length === 0,
    "typechain/**/*.ts fixture produces zero lint messages (ignored)"
  )
} finally {
  rmSync(join(pkgRoot, sharedTestFixture), { force: true })
  rmSync(join(pkgRoot, ignoredFixture), { force: true })
}

if (failures > 0) {
  console.error(`\n${failures} policy assertion(s) failed.`)
  process.exit(1)
} else {
  console.log("\nAll ESLint policy assertions passed.")
  process.exit(0)
}
