import fs from "fs"
import os from "os"
import path from "path"

import { expect } from "chai"

import resolveRandomBeaconExport, {
  resolveRandomBeaconExportIn,
} from "../../utils/random-beacon-export"

const temporaryRoots: string[] = []

function temporaryRoot(...subdirectories: string[]): string {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "beacon-export-"))
  temporaryRoots.push(root)
  subdirectories.forEach((subdirectory) => {
    fs.mkdirSync(path.join(root, subdirectory), { recursive: true })
  })
  return root
}

// utils/ sits inside the package, so the source root is the package root.
function sourceCheckout(root: string): string {
  const packageRoot = path.join(root, "ecdsa")
  fs.mkdirSync(packageRoot, { recursive: true })
  fs.writeFileSync(path.join(packageRoot, "hardhat.config.ts"), "")
  return packageRoot
}

function record(logs: string[]): (message: string) => void {
  return (message) => {
    logs.push(message)
  }
}

describe("resolveRandomBeaconExport", () => {
  const originalExportPath = process.env.RANDOM_BEACON_EXPORT_PATH

  beforeEach(() => {
    delete process.env.RANDOM_BEACON_EXPORT_PATH
  })

  afterEach(() => {
    if (originalExportPath === undefined) {
      delete process.env.RANDOM_BEACON_EXPORT_PATH
    } else {
      process.env.RANDOM_BEACON_EXPORT_PATH = originalExportPath
    }
    temporaryRoots.splice(0).forEach((root) => {
      fs.rmSync(root, { recursive: true, force: true })
    })
  })

  it("resolves the configured export path", () => {
    const root = temporaryRoot("export/deploy")
    process.env.RANDOM_BEACON_EXPORT_PATH = path.join(root, "export")
    const logs: string[] = []
    const resolved = resolveRandomBeaconExportIn(
      "deploy",
      { sourceRoot: root, packageRoot: root },
      record(logs),
    )
    expect(resolved).to.equal(path.join(root, "export", "deploy"))
    expect(logs).to.deep.equal([
      `Random Beacon deploy from RANDOM_BEACON_EXPORT_PATH: ${resolved}`,
    ])
  })

  it("throws when the configured path misses the export", () => {
    const root = temporaryRoot("export/deploy")
    process.env.RANDOM_BEACON_EXPORT_PATH = path.join(root, "export")
    const missingArtifacts = () =>
      resolveRandomBeaconExportIn(
        "artifacts",
        { sourceRoot: root, packageRoot: root },
        record([]),
      )
    expect(missingArtifacts).to.throw(
      /Random Beacon artifacts export is missing/,
    )
  })

  it("prefers the sibling checkout over the bundled scripts", () => {
    const root = temporaryRoot(
      "random-beacon/export/deploy",
      "ecdsa/external/random-beacon-export/deploy",
    )
    const packageRoot = sourceCheckout(root)
    const logs: string[] = []
    const resolved = resolveRandomBeaconExportIn(
      "deploy",
      { sourceRoot: packageRoot, packageRoot },
      record(logs),
    )
    expect(resolved).to.equal(path.join(root, "random-beacon/export/deploy"))
    expect(logs).to.deep.equal([
      `Random Beacon deploy from the sibling checkout: ${resolved}`,
    ])
  })

  it("throws when the sibling checkout is partially built", () => {
    const root = temporaryRoot(
      "random-beacon/export/deploy",
      "ecdsa/external/random-beacon-export/deploy",
    )
    const packageRoot = sourceCheckout(root)
    const partialSibling = () =>
      resolveRandomBeaconExportIn(
        "artifacts",
        { sourceRoot: packageRoot, packageRoot },
        record([]),
      )
    expect(partialSibling).to.throw(/missing in the sibling checkout/)
  })

  it("uses the bundled scripts without a sibling checkout", () => {
    const root = temporaryRoot("ecdsa/external/random-beacon-export/tasks")
    const packageRoot = sourceCheckout(root)
    const logs: string[] = []
    const resolved = resolveRandomBeaconExportIn(
      "tasks",
      { sourceRoot: packageRoot, packageRoot },
      record(logs),
    )
    expect(resolved).to.equal(
      path.join(packageRoot, "external/random-beacon-export/tasks"),
    )
    expect(logs).to.deep.equal([
      `Random Beacon tasks from the bundled copy: ${resolved}`,
    ])
  })

  it("never falls back to the npm package for deploy scripts", () => {
    const root = temporaryRoot("ecdsa")
    const packageRoot = path.join(root, "ecdsa")
    const missingBundle = () =>
      resolveRandomBeaconExportIn(
        "deploy",
        { sourceRoot: packageRoot, packageRoot },
        record([]),
      )
    expect(missingBundle).to.throw(/Random Beacon deploy export is missing/)
  })

  it("never resolves artifacts from the bundled export", () => {
    const root = temporaryRoot("ecdsa/external/random-beacon-export/artifacts")
    const packageRoot = path.join(root, "ecdsa")
    const logs: string[] = []
    const resolved = resolveRandomBeaconExportIn(
      "artifacts",
      { sourceRoot: packageRoot, packageRoot },
      record(logs),
    )
    expect(resolved).to.not.match(/external[\\/]random-beacon-export/)
    expect(resolved).to.equal(
      path.join(
        path.dirname(
          require.resolve("@keep-network/random-beacon/package.json"),
        ),
        "export",
        "artifacts",
      ),
    )
    expect(logs).to.deep.equal([
      `Random Beacon artifacts from the installed npm package: ${resolved}`,
    ])
  })

  it("resolves artifacts outside the bundled export", () => {
    expect(resolveRandomBeaconExport("artifacts")).to.not.match(
      /external[\\/]random-beacon-export/,
    )
  })
})
