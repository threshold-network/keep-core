#!/bin/bash
set -eou pipefail

LOG_START='\n\e[1;36m'           # new line + bold + color
LOG_END='\n\e[0m'                # new line + reset color
LOG_WARNING_START='\n\e\033[33m' # new line + bold + warning color
LOG_WARNING_END='\n\e\033[0m'    # new line + reset
DONE_START='\n\e[1;32m'          # new line + bold + green
DONE_END='\n\n\e[0m'             # new line + reset

KEEP_CORE_PATH=$PWD

BEACON_SOL_PATH="$KEEP_CORE_PATH/solidity/random-beacon"
ECDSA_SOL_PATH="$KEEP_CORE_PATH/solidity/ecdsa"
TMP="$KEEP_CORE_PATH/tmp"
OPENZEPPELIN_MANIFEST=".openzeppelin/unknown-*.json"
# This number should be no less than the highest index assigned to a named account
# specified in `hardhat.config.ts` configs across all the used projects. Note that
# account indices start from 0.
REQUIRED_ACCOUNTS_NUMBER=11

# Defaults, can be overwritten by env variables/input parameters
NETWORK_DEFAULT="development"
KEEP_ETHEREUM_PASSWORD=${KEEP_ETHEREUM_PASSWORD:-"password"}

help() {
  echo -e "\nUsage: ENV_VAR(S) $0" \
    "--network <network>" \
    "--tbtc-path <tbtc-path>" \
    "--threshold-network-path <threshold-network-path>" \
    "--skip-deployment" \
    "--skip-client-build"
  echo -e "\nEnvironment variables:\n"
  echo -e "\tKEEP_ETHEREUM_PASSWORD: The password to unlock local Ethereum accounts to set up delegations." \
    "Required only for 'local' network. Default value is 'password'"
  echo -e "\nCommand line arguments:\n"
  echo -e "\t--network: Ethereum network for keep-core client(s)." \
    "Available networks and settings are specified in the 'hardhat.config.ts'"
  echo -e "\t--tbtc-path: 'Local' tbtc project's path. 'tbtc' is cloned to a temporary directory" \
    "upon installation if the path is not provided"
  echo -e "\t--threshold-network-path: 'Local' threshold network project's path. 'threshold-network/solidity-contracts'" \
    "is cloned to a temporary directory upon installation if the path is not provided"
  echo -e "\t--skip-deployment: This option skips all the contracts deployment. Default is false"
  echo -e "\t--skip-client-build: Should execute contracts part only. Client installation will not be executed\n"
  exit 1 # Exit script after printing help
}

# Transform long options to short ones
for arg in "$@"; do
  shift
  case "$arg" in
  "--network") set -- "$@" "-n" ;;
  "--tbtc-path") set -- "$@" "-t" ;;
  "--threshold-network-path") set -- "$@" "-p" ;;
  "--skip-deployment") set -- "$@" "-e" ;;
  "--skip-client-build") set -- "$@" "-b" ;;
  "--help") set -- "$@" "-h" ;;
  *) set -- "$@" "$arg" ;;
  esac
done

# Parse short options
OPTIND=1
while getopts "n:t:p:ebh" opt; do
  case "$opt" in
  n) network="$OPTARG" ;;
  t) tbtc_path="$OPTARG" ;;
  p) threshold_network_path="$OPTARG" ;;
  e) skip_deployment=${OPTARG:-true} ;;
  b) skip_client_build=${OPTARG:-true} ;;
  h) help ;;
  ?) help ;; # Print help in case parameter is non-existent
  esac
done
shift $(expr $OPTIND - 1) # remove options from positional parameters

# Overwrite default properties
NETWORK=${network:-$NETWORK_DEFAULT}
TBTC_PATH=${tbtc_path:-""}
THRESHOLD_PATH=${threshold_network_path:-""}
SKIP_DEPLOYMENT=${skip_deployment:-false}
SKIP_CLIENT_BUILD=${skip_client_build:-false}

# Run script
printf "${LOG_START}Starting installation...${LOG_END}"

printf "${LOG_WARNING_START}Make sure you have at least ${REQUIRED_ACCOUNTS_NUMBER} ethereum accounts${LOG_WARNING_END}"

printf "Network: $NETWORK\n"

cd $BEACON_SOL_PATH

printf "${LOG_START}Installing beacon YARN dependencies...${LOG_END}"
yarn install --mode=update-lockfile && yarn install

if [ "$NETWORK" == "development" ]; then
  printf "${LOG_START}Unlocking ethereum accounts...${LOG_END}"
  KEEP_ETHEREUM_PASSWORD=$KEEP_ETHEREUM_PASSWORD \
    npx hardhat unlock-accounts --network $NETWORK
fi

if [ "$SKIP_DEPLOYMENT" != true ]; then

  # create tmp/ dir for fresh installations
  rm -rf $TMP && mkdir $TMP

  if [ "$THRESHOLD_PATH" = "" ]; then
    cd $TMP
    printf "${LOG_START}Cloning threshold-network/solidity-contracts...${LOG_END}"
    # clone threshold-network/solidity-contracts as a dependency for beacon, ecdsa
    # and tbtc
    git clone https://github.com/threshold-network/solidity-contracts.git

    THRESHOLD_SOL_PATH="$(realpath ./solidity-contracts)"
  else
    printf "${LOG_START}Installing threshold-network/solidity-contracts from the existing local directory...${LOG_END}"
    THRESHOLD_SOL_PATH="$THRESHOLD_PATH"
  fi

  cd "$THRESHOLD_SOL_PATH"

  printf "${LOG_START}Building threshold-network/solidity-contracts...${LOG_END}"
  yarn install --mode=update-lockfile && yarn install && yarn clean && yarn build

  # For Geth 1.16+, extract and configure private keys (personal namespace deprecated)
  if [ "$NETWORK" == "development" ]; then
    printf "${LOG_START}Extracting private keys for Hardhat (Geth 1.16+ compatibility)...${LOG_END}"
    # Try to find keystore directory
    GETH_KEYSTORE_DIR=""
    # Build list of potential keystore directories
    KEYSTORE_DIRS=()
    # Expand GETH_DATA_DIR if it's set (handles ~ expansion)
    if [ -n "${GETH_DATA_DIR:-}" ]; then
      EXPANDED_GETH_DATA_DIR=$(eval echo "$GETH_DATA_DIR")
      KEYSTORE_DIRS+=("${EXPANDED_GETH_DATA_DIR}/keystore")
    fi
    # Fallback to standard locations
    KEYSTORE_DIRS+=("$HOME/ethereum/data/keystore")
    
    for dir in "${KEYSTORE_DIRS[@]}"; do
      if [ -d "$dir" ] 2>/dev/null; then
        GETH_KEYSTORE_DIR="$dir"
        printf "Found keystore directory: $GETH_KEYSTORE_DIR\n"
        break
      fi
    done
    
    if [ -n "$GETH_KEYSTORE_DIR" ] && [ -d "$GETH_KEYSTORE_DIR" ]; then
      export DEV_ACCOUNTS_PRIVATE_KEYS=$(cd "$THRESHOLD_SOL_PATH" && node -e "
        const fs = require('fs');
        const path = require('path');
        const { ethers } = require('ethers');
        
        async function extract() {
          const keystoreDir = '$GETH_KEYSTORE_DIR';
          const passwords = ['threshold', '$KEEP_ETHEREUM_PASSWORD', 'password', ''];
          const files = fs.readdirSync(keystoreDir).filter(f => f.startsWith('UTC--'));
          const keys = [];
          
          for (const file of files.slice(0, 11)) {
            let extracted = false;
            for (const pwd of passwords) {
              try {
                const keystore = JSON.parse(fs.readFileSync(path.join(keystoreDir, file), 'utf8'));
                const wallet = await ethers.Wallet.fromEncryptedJson(JSON.stringify(keystore), pwd);
                keys.push(wallet.privateKey);
                extracted = true;
                break;
              } catch (e) {
                // Try next password
              }
            }
            if (!extracted) {
              console.error('Failed to extract key from', file);
            }
          }
          console.log(keys.join(','));
        }
        
        extract().catch((e) => {
          console.error('Extraction error:', e.message);
          process.exit(1);
        });
      " 2>&1)
      
      if [ -n "$DEV_ACCOUNTS_PRIVATE_KEYS" ] && [ "$DEV_ACCOUNTS_PRIVATE_KEYS" != "null" ]; then
        KEY_COUNT=$(echo "$DEV_ACCOUNTS_PRIVATE_KEYS" | tr ',' '\n' | grep -c . || echo "0")
        printf "Extracted $KEY_COUNT private keys\n"
        
        # Inject accounts into hardhat.config.ts for Geth 1.16+ compatibility
        if [ -f "$THRESHOLD_SOL_PATH/hardhat.config.ts" ]; then
          if ! grep -q "DEV_ACCOUNTS_PRIVATE_KEYS" "$THRESHOLD_SOL_PATH/hardhat.config.ts"; then
            printf "Configuring Hardhat config with private keys...\n"
            cd "$THRESHOLD_SOL_PATH" && node -e "
              const fs = require('fs');
              let config = fs.readFileSync('hardhat.config.ts', 'utf8');
              
              // Inject accounts into development network
              const accountsLine = '      accounts: process.env.DEV_ACCOUNTS_PRIVATE_KEYS ? process.env.DEV_ACCOUNTS_PRIVATE_KEYS.split(\",\") : undefined,';
              
              // Find development config and add accounts
              config = config.replace(
                /(development:\s*\{[^\}]*?chainId:\s*1101,)/,
                '\$1\n' + accountsLine
              );
              
              fs.writeFileSync('hardhat.config.ts', config);
              console.log('Updated hardhat.config.ts');
            " 2>/dev/null || true
          fi
        fi
      else
        printf "${LOG_WARNING_START}Warning: Could not extract private keys from $GETH_KEYSTORE_DIR. Hardhat may not be able to sign transactions with Geth 1.16+.${LOG_WARNING_END}\n"
        printf "Debug: DEV_ACCOUNTS_PRIVATE_KEYS='$DEV_ACCOUNTS_PRIVATE_KEYS'\n"
      fi
    else
      printf "${LOG_WARNING_START}Warning: Keystore directory not found. Tried: ${KEYSTORE_DIRS[*]}. Hardhat may not be able to sign transactions with Geth 1.16+.${LOG_WARNING_END}\n"
    fi
  fi

  # deploy threshold-network/solidity-contracts
  printf "${LOG_START}Deploying threshold-network/solidity-contracts contracts...${LOG_END}"
  yarn deploy --reset --network $NETWORK

  # Link the package. Replace existing link (see: https://github.com/yarnpkg/yarn/issues/7216)
  yarn unlink || true && yarn link
  # create export folder
  yarn prepack

  cd $BEACON_SOL_PATH

  # Update resolutions in package.json to handle OpenZeppelin version conflict
  # This must be done after threshold-network is cloned and before linking
  printf "${LOG_START}Updating package resolutions to resolve dependency conflicts...${LOG_END}"
  if [ -f "package.json" ] && [ -n "$THRESHOLD_SOL_PATH" ]; then
    THRESHOLD_PORTAL_PATH="portal:$THRESHOLD_SOL_PATH"
    node -e "
      const fs = require('fs');
      const pkg = JSON.parse(fs.readFileSync('package.json', 'utf8'));
      if (!pkg.resolutions) pkg.resolutions = {};
      pkg.resolutions['@threshold-network/solidity-contracts'] = '$THRESHOLD_PORTAL_PATH';
      pkg.resolutions['@openzeppelin/contracts'] = '4.7.3';
      fs.writeFileSync('package.json', JSON.stringify(pkg, null, 2) + '\n');
    " 2>/dev/null || true
    # Reinstall dependencies to apply resolutions
    printf "${LOG_START}Reinstalling dependencies with updated resolutions...${LOG_END}"
    yarn install --mode=update-lockfile
  fi

  printf "${LOG_START}Linking threshold-network/solidity-contracts...${LOG_END}"
  # Ensure we're not accidentally in the threshold directory
  CURRENT_DIR=$(realpath "$PWD" 2>/dev/null || echo "$PWD")
  THRESHOLD_DIR=$(realpath "$THRESHOLD_SOL_PATH" 2>/dev/null || echo "$THRESHOLD_SOL_PATH")
  if [ "$CURRENT_DIR" == "$THRESHOLD_DIR" ]; then
    printf "${LOG_WARNING_START}ERROR: Cannot link package to itself. Current directory is threshold-network/solidity-contracts.${LOG_WARNING_END}\n"
    exit 1
  fi
  
  # Update resolutions in package.json to handle OpenZeppelin version conflict
  if [ -f "package.json" ]; then
    # Update the portal path dynamically and add OpenZeppelin resolution
    THRESHOLD_PORTAL_PATH="portal:$THRESHOLD_SOL_PATH"
    node -e "
      const fs = require('fs');
      const pkg = JSON.parse(fs.readFileSync('package.json', 'utf8'));
      if (!pkg.resolutions) pkg.resolutions = {};
      pkg.resolutions['@threshold-network/solidity-contracts'] = '$THRESHOLD_PORTAL_PATH';
      pkg.resolutions['@openzeppelin/contracts'] = '4.7.3';
      fs.writeFileSync('package.json', JSON.stringify(pkg, null, 2) + '\n');
    " 2>/dev/null || true
    # Reinstall dependencies to apply resolutions
    printf "${LOG_START}Reinstalling dependencies with updated resolutions...${LOG_END}"
    yarn install --mode=update-lockfile && yarn install
  fi
  
  # Unlink any existing link first
  yarn unlink @threshold-network/solidity-contracts 2>/dev/null || true
  # Link to the threshold package
  yarn link "@threshold-network/solidity-contracts" || {
    printf "${LOG_WARNING_START}Failed to link @threshold-network/solidity-contracts. Trying alternative method...${LOG_WARNING_END}\n"
    yarn link "$THRESHOLD_SOL_PATH" || {
      printf "${LOG_WARNING_START}ERROR: Could not link threshold-network/solidity-contracts${LOG_WARNING_END}\n"
      exit 1
    }
  }

  printf "${LOG_START}Building random-beacon...${LOG_END}"
  yarn clean && yarn build

  # deploy beacon
  printf "${LOG_START}Deploying random-beacon contracts...${LOG_END}"
  yarn deploy --reset --network $NETWORK

  printf "${LOG_START}Creating random-beacon link...${LOG_END}"
  # Link the package. Replace existing link (see: https://github.com/yarnpkg/yarn/issues/7216)
  yarn unlink || true && yarn link
  # create export folder
  yarn prepack

  cd $ECDSA_SOL_PATH
  # remove openzeppelin manifest for fresh installation
  rm -rf $OPENZEPPELIN_MANIFEST

  # Update resolutions in package.json to handle OpenZeppelin version conflict
  printf "${LOG_START}Updating package resolutions to resolve dependency conflicts...${LOG_END}"
  if [ -f "package.json" ] && [ -n "$THRESHOLD_SOL_PATH" ]; then
    THRESHOLD_PORTAL_PATH="portal:$THRESHOLD_SOL_PATH"
    node -e "
      const fs = require('fs');
      const pkg = JSON.parse(fs.readFileSync('package.json', 'utf8'));
      if (!pkg.resolutions) pkg.resolutions = {};
      pkg.resolutions['@threshold-network/solidity-contracts'] = '$THRESHOLD_PORTAL_PATH';
      pkg.resolutions['@openzeppelin/contracts'] = '4.7.3';
      fs.writeFileSync('package.json', JSON.stringify(pkg, null, 2) + '\n');
    " 2>/dev/null || true
    # Reinstall dependencies to apply resolutions
    printf "${LOG_START}Reinstalling dependencies with updated resolutions...${LOG_END}"
    yarn install --mode=update-lockfile && yarn install
  fi

  printf "${LOG_START}Linking solidity-contracts...${LOG_END}"
  # Ensure we're not accidentally in the threshold directory
  CURRENT_DIR=$(realpath "$PWD" 2>/dev/null || echo "$PWD")
  THRESHOLD_DIR=$(realpath "$THRESHOLD_SOL_PATH" 2>/dev/null || echo "$THRESHOLD_SOL_PATH")
  if [ "$CURRENT_DIR" == "$THRESHOLD_DIR" ]; then
    printf "${LOG_WARNING_START}ERROR: Cannot link package to itself. Current directory is threshold-network/solidity-contracts.${LOG_WARNING_END}\n"
    exit 1
  fi
  
  # With portal resolution, yarn link may conflict. Try link but don't fail if it errors
  # Portal resolutions should handle the dependency resolution automatically
  yarn unlink @threshold-network/solidity-contracts 2>/dev/null || true
  
  # Try to link, but catch the "Can't link to itself" error specifically
  LINK_OUTPUT=$(yarn link "@threshold-network/solidity-contracts" 2>&1)
  LINK_EXIT=$?
  
  if echo "$LINK_OUTPUT" | grep -q "Can't link the project to itself"; then
    printf "${LOG_WARNING_START}Yarn link skipped - portal resolution handles dependencies automatically${LOG_WARNING_END}\n"
  elif [ $LINK_EXIT -ne 0 ]; then
    # Try alternative method
    ALT_LINK_OUTPUT=$(yarn link "$THRESHOLD_SOL_PATH" 2>&1)
    ALT_LINK_EXIT=$?
    if echo "$ALT_LINK_OUTPUT" | grep -q "Can't link the project to itself"; then
      printf "${LOG_WARNING_START}Yarn link not needed with portal resolution. Continuing...${LOG_WARNING_END}\n"
    elif [ $ALT_LINK_EXIT -ne 0 ]; then
      printf "${LOG_WARNING_START}Link failed, but portal resolution should handle dependencies. Continuing...${LOG_WARNING_END}\n"
    fi
  fi

  printf "${LOG_START}Linking random-beacon...${LOG_END}"
  yarn unlink @keep-network/random-beacon 2>/dev/null || true
  yarn link @keep-network/random-beacon

  printf "${LOG_START}Building ecdsa...${LOG_END}"
  yarn install --mode=update-lockfile && yarn install && yarn clean && yarn build

  # deploy ecdsa
  printf "${LOG_START}Deploying ecdsa contracts...${LOG_END}"
  yarn deploy --reset --network $NETWORK

  printf "${LOG_START}Creating ecdsa link...${LOG_END}"
  # Link the package. Replace existing link (see: https://github.com/yarnpkg/yarn/issues/7216)
  yarn unlink || true && yarn link
  # create export folder
  yarn prepack

  if [ "$TBTC_PATH" = "" ]; then
    cd $TMP
    printf "${LOG_START}Cloning tbtc...${LOG_END}"
    git clone https://github.com/keep-network/tbtc-v2.git

    TBTC_SOL_PATH="$(realpath ./tbtc-v2/solidity)"
  else
    printf "${LOG_START}Installing tbtc from the existing local directory...${LOG_END}"

    TBTC_SOL_PATH="$TBTC_PATH/solidity"
  fi

  cd "$TBTC_SOL_PATH"

  yarn install --mode=update-lockfile && yarn install

  # Update resolutions if needed
  if [ -f "package.json" ] && [ -n "$THRESHOLD_SOL_PATH" ]; then
    THRESHOLD_PORTAL_PATH="portal:$THRESHOLD_SOL_PATH"
    node -e "
      const fs = require('fs');
      const pkg = JSON.parse(fs.readFileSync('package.json', 'utf8'));
      if (!pkg.resolutions) pkg.resolutions = {};
      pkg.resolutions['@threshold-network/solidity-contracts'] = '$THRESHOLD_PORTAL_PATH';
      pkg.resolutions['@openzeppelin/contracts'] = '4.7.3';
      fs.writeFileSync('package.json', JSON.stringify(pkg, null, 2) + '\n');
    " 2>/dev/null || true
    yarn install --mode=update-lockfile && yarn install 2>/dev/null || true
  fi

  printf "${LOG_START}Linking threshold-network/solidity-contracts...${LOG_END}"
  CURRENT_DIR=$(realpath "$PWD" 2>/dev/null || echo "$PWD")
  THRESHOLD_DIR=$(realpath "$THRESHOLD_SOL_PATH" 2>/dev/null || echo "$THRESHOLD_SOL_PATH")
  if [ "$CURRENT_DIR" != "$THRESHOLD_DIR" ]; then
    yarn unlink @threshold-network/solidity-contracts 2>/dev/null || true
    yarn link "@threshold-network/solidity-contracts" || {
      printf "${LOG_WARNING_START}Failed to link @threshold-network/solidity-contracts. Trying alternative method...${LOG_WARNING_END}\n"
      yarn link "$THRESHOLD_SOL_PATH" || {
        printf "${LOG_WARNING_START}Warning: Could not link threshold-network/solidity-contracts, continuing anyway...${LOG_WARNING_END}\n"
      }
    }
  else
    printf "${LOG_WARNING_START}Skipping link - already in threshold-network directory${LOG_WARNING_END}\n"
  fi

  printf "${LOG_START}Linking random-beacon...${LOG_END}"
  yarn unlink @keep-network/random-beacon 2>/dev/null || true
  yarn link @keep-network/random-beacon

  printf "${LOG_START}Linking ecdsa...${LOG_END}"
  yarn unlink @keep-network/ecdsa 2>/dev/null || true
  yarn link @keep-network/ecdsa

  printf "${LOG_START}Building tbtc contracts...${LOG_END}"
  yarn build

  # deploy tbtc
  printf "${LOG_START}Deploying tbtc contracts...${LOG_END}"
  yarn deploy --reset --network $NETWORK
  # create export folder
  yarn prepack
fi

if [ "$SKIP_CLIENT_BUILD" = false ]; then
  printf "${LOG_START}Building client...${LOG_END}"

  cd $KEEP_CORE_PATH
  make local \
    local_beacon_path=$BEACON_SOL_PATH \
    local_ecdsa_path=$ECDSA_SOL_PATH \
    local_threshold_path=$THRESHOLD_SOL_PATH \
    local_tbtc_path=$TBTC_SOL_PATH
fi

printf "${DONE_START}Installation completed!${DONE_END}"
