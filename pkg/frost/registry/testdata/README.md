# FROST DKG Digest Fixture

`v4_digest_fixture.json` pins the keep-core Go digest implementation against
the tBTC TypeScript reference in
`contracts/tbtc-v2/test/integration/utils/frost-wallet-registry.ts`.

Regenerate the hash fields from the sibling `tlabs-xyz/tbtc` checkout:

```sh
cd ../tbtc/contracts/tbtc-v2
pnpm exec ts-node -e 'import hre from "hardhat"; import { computeFrostResultDigest } from "./test/integration/utils/frost-wallet-registry"; const { ethers } = hre; const members = [101,202,303,404,505]; const misbehavedMembersIndices = [2,5]; const activeMembers = members.filter((_, i) => !misbehavedMembersIndices.includes(i + 1)); const digest = computeFrostResultDigest(hre, { chainId: 31337, bridge: "0x1111111111111111111111111111111111111111", registry: "0x2222222222222222222222222222222222222222", seed: 123456789, xOnlyOutputKey: "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", members, misbehavedMembersIndices }); console.log(JSON.stringify({ fullMembersHash: ethers.utils.keccak256(ethers.utils.defaultAbiCoder.encode(["uint32[]"], [members])), activeMembersHash: ethers.utils.keccak256(ethers.utils.defaultAbiCoder.encode(["uint32[]"], [activeMembers])), digest, ethereumSignedMessageHash: ethers.utils.hashMessage(ethers.utils.arrayify(digest)) }, null, 2));'
```

The fixture metadata also declares the intended tBTC mirror path. The paired
tBTC-side emitter should produce byte-for-byte equivalent hash values for the
same inputs.
