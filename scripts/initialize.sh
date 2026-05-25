#!/bin/bash
set -eo pipefail

LOG_START='\n\e[1;36m'  # new line + bold + color
LOG_END='\n\e[0m'       # new line + reset color
DONE_START='\n\e[1;32m' # new line + bold + green
DONE_END='\n\n\e[0m'    # new line + reset

KEEP_CORE_PATH=$PWD
CONFIG_DIR_DEFAULT="$KEEP_CORE_PATH/configs"
KEEP_BEACON_SOL_PATH="$KEEP_CORE_PATH/solidity/random-beacon"
KEEP_ECDSA_SOL_PATH="$KEEP_CORE_PATH/solidity/ecdsa"

# Defaults, can be overwritten by env variables/input parameters
NETWORK_DEFAULT="development"

help() {
   echo -e "\nUsage: $0" \
      "--network <network>" \
      "--config-dir <path to configuration files>" \
      "--stake-owner <stake owner address>" \
      "--staking-provider <staking provider address>" \
      "--operator <operator address>" \
      "--beneficiary <beneficiary address>" \
      "--authorizer <authorizer address>" \
      "--stake-amount <stake amount>" \
      "--authorization-amount <authorization amount>"
   echo -e "\nOptional line arguments:\n"
   echo -e "\t--stake-owner: Stake owner address"
   echo -e "\t--config-dir: Path to configuration files. Default /configs"
   echo -e "\t--network: Ethereum network for keep-core client." \
      "Available networks and settings are specified in the 'hardhat.config.ts'"
   echo -e "\t--staking-provider: Staking provider address"
   echo -e "\t--operator: Operator address"
   echo -e "\t--beneficiary: Staking beneficiary address"
   echo -e "\t--authorizer: Staking authorizer address"
   echo -e "\t--stake-amount: Stake amount"
   echo -e "\t--authorization-amount: Authorization amount"
   exit 1 # Exit script after printing help
}

# Transform long options to short ones
for arg in "$@"; do
   shift
   case "$arg" in
   "--network") set -- "$@" "-n" ;;
   "--config-dir") set -- "$@" "-c" ;;
   "--stake-owner") set -- "$@" "-o" ;;
   "--staking-provider") set -- "$@" "-p" ;;
   "--operator") set -- "$@" "-d" ;;
   "--beneficiary") set -- "$@" "-b" ;;
   "--authorizer") set -- "$@" "-a" ;;
   "--stake-amount") set -- "$@" "-s" ;;
   "--authorization-amount") set -- "$@" "-k" ;;
   "--help") set -- "$@" "-h" ;;
   *) set -- "$@" "$arg" ;;
   esac
done

# Parse short options
OPTIND=1
while getopts "n:c:o:p:d:b:a:s:k:h" opt; do
   case "$opt" in
   n) network="$OPTARG" ;;
   c) config_dir="$OPTARG" ;;
   o) stake_owner="$OPTARG" ;;
   p) staking_provider="$OPTARG" ;;
   d) operator="$OPTARG" ;;
   b) beneficiary="$OPTARG" ;;
   a) authorizer="$OPTARG" ;;
   s) stake_amount="$OPTARG" ;;
   k) authorization_amount="$OPTARG" ;;
   h) help ;;
   ?) help ;; # Print help in case parameter is non-existent
   esac
done
shift $(expr $OPTIND - 1) # remove options from positional parameters

CONFIG_DIR=${config_dir:-$CONFIG_DIR_DEFAULT}

# read from the config file if a stake_owner is not provided as parameter
if [ -z "$stake_owner" ]; then
   printf "\nReading stake owner address from the config file..."

   # read from the config file when the stake owner is not provided
   config_files=($CONFIG_DIR/*.toml)
   config_files_count=${#config_files[@]}
   while :; do
      printf "\nSelect client config file: \n"
      i=1
      for o in "${config_files[@]}"; do
         echo "$i) ${o##*/}"
         let i++
      done

      read reply
      if [ "$reply" -ge 1 ] && [ "$reply" -le $config_files_count ]; then
         CONFIG_FILE_PATH=${config_files["$reply" - 1]}
         break
      else
         printf "\nInvalid choice. Please choose an existing option number.\n"
      fi
   done

   key_file_path=$(awk '/[k|K]eyFile ?=(.*)$/{print $3}' $CONFIG_FILE_PATH | xargs)

   printf "Reading an address from the key file: ${key_file_path}\n"

   stake_owner=`cat $key_file_path | jq -jr .address`

   [[ $stake_owner == 0x* ]] || stake_owner="0x$stake_owner"

   printf "Discovered the address: ${stake_owner}\n"
fi

# Overwrite default properties
NETWORK=${network:-$NETWORK_DEFAULT}

if [ -z "$staking_provider" ]; then
   staking_provider=${stake_owner}
fi

if [ -z "$operator" ]; then
   operator=${stake_owner}
fi

if [ -z "$beneficiary" ]; then
   beneficiary=${stake_owner}
fi

if [ -z "$authorizer" ]; then
   authorizer=${stake_owner}
fi

initialize=(
   npx hardhat initialize
   --network "$NETWORK"
   --owner "$stake_owner"
   --provider "$staking_provider"
   --operator "$operator"
   --beneficiary "$beneficiary"
   --authorizer "$authorizer"
)

if [ ! -z "$stake_amount" ]; then
   initialize+=(--amount "$stake_amount")
fi

if [ ! -z "$authorization_amount" ]; then
   initialize+=(--authorization "$authorization_amount")
fi

printf "${LOG_START}Initializing beacon...${LOG_END}"
cd "$KEEP_BEACON_SOL_PATH"
"${initialize[@]}"

printf "${LOG_START}Initializing ecdsa...${LOG_END}"
cd "$KEEP_ECDSA_SOL_PATH"
"${initialize[@]}"

printf "${DONE_START}Initialization completed!${DONE_END}"
