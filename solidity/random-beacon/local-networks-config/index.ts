// Vendored from @keep-network/hardhat-local-networks-config@0.1.0-pre.4
// (https://github.com/keep-network/hardhat-local-networks-config, itself a
// fork of https://github.com/facuspagnuolo/hardhat-local-networks-config-plugin),
// which is dead upstream (4.5+ years frozen, no further releases). Vendored
// directly to drop the cross-org dependency; see
// threshold-network/keep-common's dependency plan for rationale. Do not add
// new features here beyond what upstream published — this is a frozen copy.
import fs from "fs"
import path from "path"
import { homedir } from "os"
import deepmerge from "deepmerge"
import { extendConfig } from "hardhat/config"
import {
  HardhatConfig,
  NetworkConfig,
  NetworksUserConfig,
  NetworkUserConfig,
  HardhatUserConfig,
} from "hardhat/types"
import { HardhatPluginError } from "hardhat/plugins"
import { parseLocalNetworksConfigPath } from "./utils"
import "./type-extensions"

const HARDHAT_CONFIG_DIR = ".hardhat"
const HARDHAT_NETWORK_CONFIG_FILE = "networks.json"

export interface LocalNetworksConfig {
  networks?: NetworksUserConfig
  defaultConfig?: NetworkUserConfig
}

interface LocalNetworksConfigInternal extends LocalNetworksConfig {
  networks: NetworksUserConfig
  defaultConfig: NetworkUserConfig
}

extendConfig(
  (hardhatConfig: HardhatConfig, userConfig: HardhatUserConfig): void => {
    const homeLocalNetworksConfig = readHomeNetworksConfig()
    const localNetworksConfig = readLocalNetworksConfig(
      hardhatConfig,
      userConfig
    )

    const userNetworkConfigs: NetworksUserConfig = userConfig.networks || {}
    Object.entries(userNetworkConfigs).forEach(
      ([networkName, userNetworkConfig]) => {
        hardhatConfig.networks[networkName] = deepmerge.all([
          hardhatConfig.networks[networkName] || {},
          homeLocalNetworksConfig.defaultConfig,
          homeLocalNetworksConfig.networks[networkName] || {},
          localNetworksConfig.defaultConfig,
          userNetworkConfig as object,
          localNetworksConfig.networks[networkName] || {},
        ]) as NetworkConfig
      }
    )

    Object.entries(localNetworksConfig.networks).forEach(
      ([networkName, localNetworkConfig]) => {
        if (!hardhatConfig.networks[networkName]) {
          hardhatConfig.networks[networkName] = deepmerge.all([
            hardhatConfig.networks[networkName] || {},
            homeLocalNetworksConfig.defaultConfig,
            localNetworksConfig.defaultConfig,
            localNetworkConfig as object,
          ]) as NetworkConfig
        }
      }
    )

    Object.entries(homeLocalNetworksConfig.networks).forEach(
      ([networkName, localNetworkConfig]) => {
        if (!hardhatConfig.networks[networkName]) {
          hardhatConfig.networks[networkName] = deepmerge.all([
            hardhatConfig.networks[networkName] || {},
            homeLocalNetworksConfig.defaultConfig,
            localNetworksConfig.defaultConfig,
            localNetworkConfig as object,
          ]) as NetworkConfig
        }
      }
    )
  }
)

export function readHomeNetworksConfig(): LocalNetworksConfigInternal {
  const configPath = getDefaultHomeLocalNetworksConfigPath()
  const networksConfig = fs.existsSync(configPath) ? require(configPath) : {}

  if (!networksConfig.networks) networksConfig.networks = []
  if (!networksConfig.defaultConfig) networksConfig.defaultConfig = {}

  return networksConfig
}

export function readLocalNetworksConfig(
  hardhatConfig: HardhatConfig,
  userConfig: HardhatUserConfig
): LocalNetworksConfigInternal {
  const localNetworksConfigPath = parseLocalNetworksConfigPath(
    userConfig,
    hardhatConfig.paths.root
  )

  if (localNetworksConfigPath && !fs.existsSync(localNetworksConfigPath)) {
    throw new HardhatPluginError(
      `hardhat-local-networks-config-plugin`,
      `configuration file not found under "localNetworksConfig" path: ${userConfig.localNetworksConfig}; ` +
        `resolved path: ${localNetworksConfigPath}`
    )
  }

  const localNetworksConfig = localNetworksConfigPath
    ? require(localNetworksConfigPath)
    : {}

  if (!localNetworksConfig.networks) localNetworksConfig.networks = []
  if (!localNetworksConfig.defaultConfig) localNetworksConfig.defaultConfig = {}

  return localNetworksConfig
}

export function getDefaultHomeLocalNetworksConfigPath() {
  return path.join(getHomeConfigDir(), HARDHAT_NETWORK_CONFIG_FILE)
}

export function getHomeConfigDir() {
  return path.join(homedir(), HARDHAT_CONFIG_DIR)
}
