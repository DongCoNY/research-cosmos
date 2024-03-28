#!/bin/bash

PASSPHRASE="12345678"
Description="XX"
Title="Enable Band IBC with a request interval of 10 blocks"

TX_OPTS="--from=genesis  --chain-id=injective-1 --gas-prices=500000000inj --broadcast-mode=sync --yes"

yes $PASSPHRASE | injectived tx oracle enable-band-ibc-proposal true 10 --port-id "oracle" --channel "channel-0" --ibc-version "bandchain-1" --title="$Title" --description="$Description" --deposit="100000000000inj" $TX_OPTS
yes $PASSPHRASE | injectived tx gov vote 1 yes $TX_OPTS

echo "😴 Sleeping for 10 sec"
sleep 5
Description="XX"
Title="Authorize Band IBC Oracle Request"

yes $PASSPHRASE | injectived tx oracle authorize-band-oracle-request-proposal 37 \
--symbols "BTC,ETH,USDT,USDC,MATIC" \
--requested-validator-count 4 \
--sufficient-validator-count 3 \
--prepare-gas 50000 \
--fee-limit "1000uband" \
--execute-gas 300000 \
--title="$Title" --description="$Description" --deposit="100000000000inj" $TX_OPTS

yes $PASSPHRASE | injectived tx gov vote 2 yes $TX_OPTS


echo "😴 Sleeping for 10 sec"
sleep 5
Description="XX"
Title="Authorize Band IBC Oracle Request2"

yes $PASSPHRASE | injectived tx oracle authorize-band-oracle-request-proposal 37 \
--symbols "MIR,ANC,DOGE,LUNA,BNB" \
--requested-validator-count 4 \
--sufficient-validator-count 3 \
--prepare-gas 50000 \
--fee-limit "1000uband" \
--execute-gas 300000 \
--title="$Title" --description="$Description" --deposit="100000000000inj" $TX_OPTS

yes $PASSPHRASE | injectived tx gov vote 3 yes $TX_OPTS


