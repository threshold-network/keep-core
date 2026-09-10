// Vendored from @keep-network/hardhat-local-networks-config@0.1.0-pre.4 —
// see index.ts header for provenance.
import "hardhat/types/config"

declare module "hardhat/types/config" {
  export interface HardhatUserConfig {
    localNetworksConfig?: string
  }
}
