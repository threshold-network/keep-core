#!/bin/bash
set -e

LOG_START='\n\e[1;36m'           # new line + bold + color
LOG_END='\n\e[0m'                # new line + reset color
LOG_WARNING_START='\n\e\033[33m' # new line + bold + warning color
LOG_WARNING_END='\n\e\033[0m'    # new line + reset
DONE_START='\n\e[1;32m'          # new line + bold + green
DONE_END='\n\n\e[0m'             # new line + reset

KEEP_CORE_PATH=$PWD

TOKEN_DASHBOARD_PATH="$KEEP_CORE_PATH/token-dashboard"

cd $TOKEN_DASHBOARD_PATH

printf "${LOG_START}Starting installation...${LOG_END}"
yarn install --ignore-scripts

printf "${LOG_START}Starting post installation...${LOG_END}"
yarn run postinstall

printf "${LOG_START}Starting dApp...${LOG_END}"
yarn start
