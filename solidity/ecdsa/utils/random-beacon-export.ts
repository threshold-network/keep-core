import fs from "fs"
import path from "path"

// This module also runs from export/utils in a packed ECDSA package.
const sourceRoot = path.resolve(__dirname, "..")
const packageRoot = fs.existsSync(path.join(sourceRoot, "package.json"))
  ? sourceRoot
  : path.dirname(sourceRoot)

export type RandomBeaconExportKind = "deploy" | "artifacts" | "tasks"

export interface RandomBeaconExportRoots {
  /** Directory holding this module's parent: a checkout or a packed export. */
  sourceRoot: string
  /** Directory holding the ECDSA package.json. */
  packageRoot: string
}

/**
 * Resolves a Random Beacon export directory and logs the producer it came
 * from. Every subdirectory comes from a single producer: the sibling checkout
 * is selected on its export root alone, so a half-built sibling fails loudly
 * instead of pairing its fresh deploy scripts with another producer's
 * artifacts.
 */
export function resolveRandomBeaconExportIn(
  subdir: RandomBeaconExportKind,
  roots: RandomBeaconExportRoots,
  log: (message: string) => void = console.log,
): string {
  // Explicit producer checks must never fall back to another package's code.
  const exportRoot = process.env.RANDOM_BEACON_EXPORT_PATH
  if (exportRoot) {
    const configured = path.resolve(exportRoot, subdir)
    if (!fs.existsSync(configured)) {
      throw new Error(
        `Random Beacon ${subdir} export is missing: ${configured}`,
      )
    }
    log(`Random Beacon ${subdir} from RANDOM_BEACON_EXPORT_PATH: ${configured}`)
    return configured
  }

  // Published packages have only export/hardhat.config.js. Without a source
  // config, the adjacent random-beacon is an npm dependency, potentially v5.
  const siblingRoot = path.join(roots.packageRoot, "../random-beacon/export")
  if (
    fs.existsSync(path.join(roots.sourceRoot, "hardhat.config.ts")) &&
    fs.existsSync(siblingRoot)
  ) {
    const sibling = path.join(siblingRoot, subdir)
    if (!fs.existsSync(sibling)) {
      throw new Error(
        `Random Beacon ${subdir} export is missing in the sibling checkout: ${sibling}`,
      )
    }
    log(`Random Beacon ${subdir} from the sibling checkout: ${sibling}`)
    return sibling
  }

  // ECDSA never bundles Beacon artifacts, so only they may come from the
  // installed package. Deploy scripts and tasks ship with this package and
  // must never be paired with a possibly v5 dependency.
  if (subdir === "artifacts") {
    const installed = path.join(
      path.dirname(require.resolve("@keep-network/random-beacon/package.json")),
      "export",
      subdir,
    )
    log(`Random Beacon ${subdir} from the installed npm package: ${installed}`)
    return installed
  }

  const bundled = path.join(
    roots.packageRoot,
    "external/random-beacon-export",
    subdir,
  )
  if (!fs.existsSync(bundled)) {
    throw new Error(`Random Beacon ${subdir} export is missing: ${bundled}`)
  }
  log(`Random Beacon ${subdir} from the bundled copy: ${bundled}`)
  return bundled
}

export default function resolveRandomBeaconExport(
  subdir: RandomBeaconExportKind,
): string {
  return resolveRandomBeaconExportIn(subdir, { sourceRoot, packageRoot })
}
