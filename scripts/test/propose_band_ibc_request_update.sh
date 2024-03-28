#!/bin/bash

PASSPHRASE="12345678"
Description="XX"
Title="Enable Band IBC with a request interval of 10 blocks"

TX_OPTS="--from=genesis  --chain-id=injective-1 --gas-prices=500000000inj --broadcast-mode=sync --yes"

Description="XX"
Title="Update Band IBC Oracle Request"

yes $PASSPHRASE | injectived tx oracle update-band-oracle-request-proposal 1 37 \
--symbols "BTC,DOGE" \
--requested-validator-count 4 \
--sufficient-validator-count 3 \
--prepare-gas 50000 \
--fee-limit "1000uband" \
--execute-gas 300000 \
--title="$Title" --description="$Description" --deposit="100000000000inj" $TX_OPTS

yes $PASSPHRASE | injectived tx gov vote 4 yes $TX_OPTS