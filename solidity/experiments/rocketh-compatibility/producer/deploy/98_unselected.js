import { deployScript } from "../rocketh.js";

export default deployScript(async () => {
  throw new Error("A script outside the selected tags must not run");
}, { tags: ["Unselected"] });
