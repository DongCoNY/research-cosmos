#sample command
# PEGGY_ADDR=0xA5aC3d1991F7C9A3968a9c317Bc68675Dfde42B8 \
# INJ_ADDR=0x5f1de74dC635604EDE186D7EfD6a931c26A37937 \
# ETH_RPC=https://kovan.injective.dev \
# ETH_CHAIN_ID=50 \
# ./ci-override-config.sh

# override peggo config
for i in $(seq 0 3)
do
  #init directory
  mkdir -p ./node${i}/peggo/

  #parse config
  COSMOS_PRIV=$(yes 12345678 | injectived keys unsafe-export-eth-key node${i} --home ./node${i})
  ETH_PRIV=NODE${i}_ETH_PRIV

  echo "PEGGO_ENV=local
PEGGO_LOG_LEVEL=debug
PEGGO_SERVICE_WAIT_TIMEOUT=1m

PEGGO_COSMOS_CHAIN_ID=injective-1
PEGGO_COSMOS_GRPC=tcp://localhost:9900
PEGGO_TENDERMINT_RPC=http://localhost:26657
PEGGO_COSMOS_FEE_DENOM=inj
PEGGO_COSMOS_GAS_PRICES=500000000inj

PEGGO_COSMOS_KEYRING=file
PEGGO_COSMOS_KEYRING_DIR=~/.peggo
PEGGO_COSMOS_KEYRING_APP=peggo
PEGGO_COSMOS_FROM=
PEGGO_COSMOS_FROM_PASSPHRASE=
PEGGO_COSMOS_PK=0x$COSMOS_PRIV
PEGGO_COSMOS_USE_LEDGER=false

PEGGO_ETH_CHAIN_ID=$ETH_CHAIN_ID
PEGGO_ETH_RPC=$ETH_RPC
PEGGO_ETH_CONTRACT_ADDRESS=$PEGGY_ADDR

PEGGO_ETH_KEYSTORE_DIR=
PEGGO_ETH_FROM=
PEGGO_ETH_PASSPHRASE=
PEGGO_ETH_PK=${!ETH_PRIV}
PEGGO_ETH_USE_LEDGER=false

PEGGO_STATSD_PREFIX=peggo.
PEGGO_STATSD_ADDR=localhost:8125
PEGGO_STATSD_STUCK_DUR=5m
PEGGO_STATSD_MOCKING=false
PEGGO_STATSD_DISABLED=true" > ./node${i}/peggo/.env

done

# override genesis.json config
PEGGY_ADDR="$PEGGY_ADDR" INJ_ADDR="$INJ_ADDR" jq '.app_state.peggy.params.bridge_ethereum_address = env.PEGGY_ADDR | .app_state.peggy.params.cosmos_coin_erc20_contract = env.INJ_ADDR' ./node0/config/genesis.json > ./node0/config/temp.json
mv ./node0/config/temp.json ./node0/config/genesis.json
cp ./node0/config/genesis.json ./node1/config/
cp ./node0/config/genesis.json ./node2/config/
cp ./node0/config/genesis.json ./node3/config/
