#!/bin/bash

PASSPHRASE="12345678"
Title="INJ/ATOM market status set"
Description="XX"
BaseDenom="inj"
QuoteDenom="atom"
Status="Paused"

TX_OPTS="--from=genesis  --chain-id=injective-1 --gas-prices=500000000inj --broadcast-mode=sync --yes"

yes $PASSPHRASE | injectived tx exchange set-spot-market-status "$BaseDenom" "$QuoteDenom" "$Status" --title="$Title" --description="$Description" --deposit="100000000000inj" $TX_OPTS
yes $PASSPHRASE | injectived tx gov vote 1 yes $TX_OPTS