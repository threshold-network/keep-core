// Automated tests for the SIGINT/SIGTERM cleanup contract in
// `test-eslint-policy.mjs`. The smoke-test script writes temporary fixture
// files under this package's test/ and typechain/ directories and depends
// on its signal handlers removing them when the process is interrupted
// before the script's `finally` block can run. These tests spawn the smoke
// test as a child process, interrupt it with SIGINT and SIGTERM mid-run,
// and verify that no `policy-fixture-*.ts` files are left behind in either
// directory. A normal-completion run is also covered so a regression that
// breaks the `finally` cleanup shows up here as well.
//
// Run from this directory with: node --test scripts/test-eslint-policy.test.mjs

import { test, beforeEach, afterEach } from "node:test"
import assert from "node:assert/strict"
import { spawn } from "node:child_process"
import { existsSync, readdirSync, rmSync } from "node:fs"
import { join } from "node:path"
import { setTimeout as sleep } from "node:timers/promises"

const scriptPath = new URL("./test-eslint-policy.mjs", import.meta.url).pathname
const projectRoot = join(import.meta.dirname, "..")
const testDir = join(projectRoot, "test")
const typechainDir = join(projectRoot, "typechain")

// The smoke test scopes every temporary fixture it creates under a
// `policy-fixture-<nonce>.ts` filename. Matching that prefix is enough to
// distinguish smoke-test fixtures from any other file the package may
// legitimately have under test/ or typechain/.
const FIXTURE_PATTERN = /^policy-fixture-.*\.ts$/

function findPolicyFixtures(dir) {
  if (!existsSync(dir)) return []
  return readdirSync(dir).filter((f) => FIXTURE_PATTERN.test(f))
}

function listFixtures() {
  return {
    test: findPolicyFixtures(testDir),
    typechain: findPolicyFixtures(typechainDir),
  }
}

function cleanupFixtures() {
  ;[testDir, typechainDir].forEach((dir) => {
    findPolicyFixtures(dir).forEach((f) => {
      rmSync(join(dir, f), { force: true })
    })
  })
}

// Poll until at least one prefixed fixture exists, then return. The smoke
// test creates the first fixture file very early in its run, so once any
// file is present the script is mid-execution and SIGINT/SIGTERM can
// interrupt it before the `finally` cleanup runs. Recursive tail call to
// keep the await outside of any loop body.
async function waitForAnyFixture(timeoutMs = 30_000, pollMs = 25) {
  const fixtures = listFixtures()
  if (fixtures.test.length > 0 || fixtures.typechain.length > 0) {
    return fixtures
  }
  if (timeoutMs <= 0) {
    return null
  }
  await sleep(pollMs)
  return waitForAnyFixture(timeoutMs - pollMs, pollMs)
}

function spawnSmokeTest() {
  return spawn(process.execPath, [scriptPath], {
    cwd: projectRoot,
    stdio: ["ignore", "pipe", "pipe"],
  })
}

function awaitExit(child) {
  if (child.exitCode !== null || child.signalCode !== null) {
    return Promise.resolve({ code: child.exitCode, signal: child.signalCode })
  }
  return new Promise((resolve) => {
    child.once("exit", (code, signal) => resolve({ code, signal }))
  })
}

function assertExitShape(actual, expectedCode, expectedSignal, message) {
  const { code, signal } = actual
  if (code !== expectedCode && signal !== expectedSignal) {
    throw new Error(
      `${message}: expected code=${expectedCode} or signal=${expectedSignal}; got code=${code} signal=${signal}`,
    )
  }
}

beforeEach(cleanupFixtures)
afterEach(cleanupFixtures)

test("normal completion exits 0 and leaves no prefixed fixtures", async (t) => {
  const child = spawnSmokeTest()
  // Guarantee no leaked process if the test times out.
  t.after(() => {
    if (child.exitCode === null && child.signalCode === null) {
      child.kill("SIGKILL")
    }
  })

  const exitInfo = await awaitExit(child)
  assertExitShape(exitInfo, 0, null, "normal completion exit")

  // Brief grace period so a lazy async filesystem flush cannot give a
  // false positive on the cleanup check.
  await sleep(100)

  assert.deepEqual(
    listFixtures(),
    { test: [], typechain: [] },
    "no prefixed fixtures should remain after normal completion",
  )
})

test("SIGINT cleans up fixtures and exits 130 (or via SIGINT signal)", async (t) => {
  const child = spawnSmokeTest()
  t.after(() => {
    if (child.exitCode === null && child.signalCode === null) {
      child.kill("SIGKILL")
    }
  })

  const observed = await waitForAnyFixture()
  assert.ok(
    observed,
    "smoke test did not create a prefixed fixture within the polling window; cannot interrupt",
  )

  child.kill("SIGINT")
  const exitInfo = await awaitExit(child)
  assertExitShape(exitInfo, 130, "SIGINT", "SIGINT exit shape")

  await sleep(100)
  assert.deepEqual(
    listFixtures(),
    { test: [], typechain: [] },
    "no prefixed fixtures should remain after SIGINT",
  )
})

test("SIGTERM cleans up fixtures and exits 143 (or via SIGTERM signal)", async (t) => {
  const child = spawnSmokeTest()
  t.after(() => {
    if (child.exitCode === null && child.signalCode === null) {
      child.kill("SIGKILL")
    }
  })

  const observed = await waitForAnyFixture()
  assert.ok(
    observed,
    "smoke test did not create a prefixed fixture within the polling window; cannot interrupt",
  )

  child.kill("SIGTERM")
  const exitInfo = await awaitExit(child)
  assertExitShape(exitInfo, 143, "SIGTERM", "SIGTERM exit shape")

  await sleep(100)
  assert.deepEqual(
    listFixtures(),
    { test: [], typechain: [] },
    "no prefixed fixtures should remain after SIGTERM",
  )
})
