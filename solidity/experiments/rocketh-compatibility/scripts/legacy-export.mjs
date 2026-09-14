import { getAddress } from "ethers";

// Prototype for deploy-v1's single-network JSON export, not its artifact task.
// Keep insertion order and checksum addresses to match a fresh v1 deployment.
export function legacyExport(env) {
  const contracts = {};
  for (const [name, deployment] of Object.entries(env.deployments)) {
    contracts[name] = {
      address: getAddress(deployment.address),
      abi: deployment.abi,
      linkedData: deployment.linkedData,
    };
  }
  return JSON.stringify({
    name: env.name,
    chainId: String(env.network.chain.id),
    contracts,
  }, null, 2);
}
