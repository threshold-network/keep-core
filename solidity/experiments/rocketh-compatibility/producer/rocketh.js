import { setupDeployScripts } from "rocketh";
import * as deploy from "@rocketh/deploy";
import * as readExecute from "@rocketh/read-execute";

export const extensions = { ...deploy, ...readExecute };
export const { deployScript } = setupDeployScripts(extensions);
