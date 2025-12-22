#!/bin/sh
set -e

# generate a random node id
export RANDOM_ID=$(cat /dev/urandom | tr -dc 'a-zA-Z0-9' | fold -w 32 | head -n 1)
echo "-- RANDOM_ID: $RANDOM_ID"
echo ""

# create new account for this Keep client
/geth account new  --password /root/passphrase | \
    cut -d "{" -f2 | cut -d "}" -f1 > /root/account0
export GETH_ETH_ACCOUNT0=`cat /root/account0`
echo "-- GETH_ETH_ACCOUNT0: $GETH_ETH_ACCOUNT0"
echo ""

# create new account for Keep peers
/geth account new  --password /root/passphrase | \
    cut -d "{" -f2 | cut -d "}" -f1 > /root/account1
export GETH_ETH_ACCOUNT1=`cat /root/account1`
echo "-- GETH_ETH_ACCOUNT1: $GETH_ETH_ACCOUNT1"
echo ""

# Generate genesis.json and issue tokens to Keep peers account1
cat <<EOF >> /root/genesis.json
{
  "config": {
    "chainId": 1101,
    "homesteadBlock": 0,
    "eip155Block": 0,
    "eip158Block": 0,
    "terminalTotalDifficulty": null
  },
  "difficulty" : "0x20000",
  "gasLimit"   : "0x493E00",
  "alloc": {
EOF

echo "   \"0x$GETH_ETH_ACCOUNT1\": {" >> /root/genesis.json
echo "    \"balance\": \"1000000000000000000000\"" >> /root/genesis.json
cat <<EOF >> /root/genesis.json
   }
  }
}
EOF

# dump genesis file
echo "-- Dump genesis.json:"
cat /root/genesis.json
echo ""

# initialize chain with our genesis.json parameters
# Only initialize if chaindata doesn't already exist
if [ ! -d "/root/.ethereum/geth/chaindata" ]; then
  echo "-- No chaindata directory. Need to Initialize. Writing genesis block..."
  /geth init /root/genesis.json
else
  echo "-- Chaindata directory already exists. Skipping initialization."
fi
echo ""

# start miner and allocate rewards to account0
echo "-- Start geth mining for account0: $GETH_ETH_ACCOUNT0"
echo ""

exec "/geth" --port 30303 --networkid 1101 \
    --ws --wsaddr "0.0.0.0" --wsport 8546 --wsorigins "*" \
    --rpc --rpcport 8545 --rpcaddr 0.0.0.0 --rpccorsdomain "" \
    --rpcapi "db,ssh,miner,admin,eth,net,web3,personal" \
    --syncmode "fast" \
    --mine --minerthreads=1 \
    --identity $RANDOM_ID \
    --miner.etherbase=$GETH_ETH_ACCOUNT0
