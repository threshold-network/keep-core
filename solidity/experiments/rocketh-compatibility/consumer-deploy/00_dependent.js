import { deployScript } from "@keep-test/rocketh-producer/rocketh";
import artifact from "../artifacts/contracts/Probe.sol/DependentProbe.json" with { type: "json" };

export default deployScript(async (env) => {
  await env.deploy("DependentProbe", {
    account: env.namedAccounts.deployer,
    artifact,
    args: [env.get("Probe").address],
  });
}, { tags: ["DependentProbe"], dependencies: ["Probe"] });
