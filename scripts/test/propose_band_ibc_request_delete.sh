#!/bin/bash

PASSPHRASE="12345678"
TX_OPTS="--from=genesis  --chain-id=injective-1 --gas-prices=500000000inj --broadcast-mode=sync --yes"

Description="XX"
Title="Delete Band IBC Oracle Request"

yes $PASSPHRASE | injectived tx oracle delete-band-oracle-request-proposal 1 --title="$Title" --description="$Description" --deposit="100000000000inj" $TX_OPTS
yes $PASSPHRASE | injectived tx gov vote 5 yes $TX_OPTS