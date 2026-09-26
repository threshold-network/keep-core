import { deployScript } from "../rocketh.js";
import artifact from "../artifacts/Probe.json" with { type: "json" };

export default deployScript(async (env) => {
  await env.deploy("Probe", {
    account: env.namedAccounts.deployer,
    artifact,
    args: [env.namedAccounts.owner, 7n],
  }, { linkedData: { purpose: "compatibility", revision: 1 } });
}, { tags: ["Probe"] });
