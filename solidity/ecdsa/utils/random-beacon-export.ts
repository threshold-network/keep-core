import fs from "fs"
import path from "path"

// This module also runs from export/utils in a packed ECDSA package.
const sourceRoot = path.resolve(__dirname, "..")
const packageRoot = fs.existsSync(path.join(sourceRoot, "package.json"))
  ? sourceRoot
  : path.dirname(sourceRoot)

export default function resolveRandomBeaconExport(
  subdir: "deploy" | "artifacts" | "tasks",
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
    return configured
  }

  // Published packages have only export/hardhat.config.js. Without a source
  // config, the adjacent random-beacon is an npm dependency, potentially v5.
  if (fs.existsSync(path.join(sourceRoot, "hardhat.config.ts"))) {
    const local = path.join(packageRoot, "../random-beacon/export", subdir)
    if (fs.existsSync(local)) {
      return local
    }
  }

  if (subdir !== "artifacts") {
    const bundled = path.join(
      packageRoot,
      "external/random-beacon-export",
      subdir,
    )
    if (fs.existsSync(bundled)) {
      return bundled
    }
  }

  return path.join(
    path.dirname(require.resolve("@keep-network/random-beacon/package.json")),
    "export",
    subdir,
  )
}
