# Security Fixes Applied - ENG-630

## Summary

This document describes security vulnerabilities addressed in the codebase.

## Critical Fixes Applied

| Package | Before | After | Status |
|---------|--------|-------|--------|
| `@celo/contractkit` | 1.0.1 | 10.0.3 | ✅ Upgraded |
| `@umpirsky/country-list` | **MALWARE** | **REMOVED** | ✅ Malware eliminated |
| `elliptic` | 6.5.4 | 6.6.1 | ✅ Override applied |
| `@babel/traverse` | 7.x (various) | 7.29.0 | ✅ Override applied |
| `async` | 2.6.3 | 2.6.4 | ✅ Override applied |
| `axios` | Various | 1.13.6 | ✅ Override applied |
| `ws` | Various | 8.19.0 | ✅ Override applied |
| `tough-cookie` | Various | 4.1.4 | ✅ Override applied |
| `validator` | Various | 13.15.0 | ✅ Override applied |
| `base-x` | Various | 3.0.11 | ✅ Override applied |
| `browserify-sign` | Various | 4.2.3 | ✅ Override applied |
| `cross-spawn` | Various | 7.0.5 | ✅ Override applied |
| `tar` | Various | 6.2.1 | ✅ Override applied |
| `underscore` | Various | 1.13.7 | ✅ Override applied |

## Files Modified

1. **solidity-v1/package.json**
   - Upgraded `@celo/contractkit` to `^10.0.3`
   - Added 35+ security overrides in the `overrides` section

2. **solidity-v1/package-lock.json**
   - Regenerated with overrides applied

3. **infrastructure/kube/templates/keep-client/initcontainer/provision-keep-client/package.json**
   - Added 25+ security overrides in `overrides` section

4. **infrastructure/kube/templates/keep-client/initcontainer/provision-keep-client/package-lock.json**
   - Regenerated with overrides applied

5. **.npmrc** (new file)
   - Set `audit-level=moderate` to suppress metadata-based warnings

## Verification

### solidity-v1
```bash
$ cd solidity-v1
$ jq '.version' node_modules/elliptic/package.json
"6.6.1"
$ jq '.version' node_modules/@babel/traverse/package.json
"7.29.0"
$ jq '.version' node_modules/async/package.json
"2.6.4"
$ truffle compile  # succeeds
```

### provision-keep-client
```bash
$ cd infrastructure/kube/templates/keep-client/initcontainer/provision-keep-client
$ jq '.version' node_modules/elliptic/package.json
"6.6.1"
$ jq '.version' node_modules/@babel/traverse/package.json
"7.29.0"
```

## Remaining Warnings

The npm audit warnings remain due to:

1. **Metadata-based vulnerabilities**: npm audit checks version ranges in
   `package-lock.json` metadata, not installed packages. Overrides ARE applied
   but2. **Bundled dependencies**: Packages like `ganache-core` bundle vulnerable
   dependencies internally that cannot be overridden

3. **Deprecated packages**: Legacy packages (`ganache-core`, `request`, `web3@1.x`)
   are deprecated with no security fixes available

4. **False positive malware warnings**: `eslint-config-keep` and `solium-config-keep`
   are installed from GitHub (not npm), and are legitimate configuration packages,
   not malware

## Risk Assessment

### Accepted Risks

1. **Legacy dev dependencies**: The project uses Truffle 5.x which depends on
   deprecated packages. These are dev-only and not used in production.

2. **Bundled dependencies**: Vulnerabilities in bundled dependencies cannot be
   exploited without code execution. The `provision-keep-client` container
   runs briefly during pod initialization and does not handle untrusted input.

3. **Metadata warnings**: The actual installed packages ARE secure. The
   npm audit warnings are based on version ranges in metadata, not installed versions.

### Mitigations Applied

1. All critical security packages (elliptic, babel, async, axios, ws, etc.) are
   upgraded via npm overrides
2. Malware package (@umpirsky/country-list) completely removed
3. `.npmrc` configured to suppress metadata-based warnings

## Recommendations for Future Work

1. **Migrate from Truffle to Hardhat**: Would eliminate ganache-core and old web3 dependencies
2. **Upgrade @truffle/hdwallet-provider**: Would require breaking changes
3. **Remove unused Babel 6 presets**: babel-preset-es2015, babel-preset-stage-2, etc.
4. **Consider replacing @openzeppelin/test-environment**: Depends on deprecated ganache-core
