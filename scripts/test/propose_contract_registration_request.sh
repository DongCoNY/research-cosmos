#!/bin/bash

PASSPHRASE="12345678"
Description="XX"
Title="Contract Registration Request Proposal"

TX_OPTS="--from=genesis  --chain-id=injective-1 --broadcast-mode=sync --yes  --fees=100000000000000000inj --gas=2500000"

yes $PASSPHRASE | injectived tx xwasm propose-contract-registration-request \
--contract-address "inj17p9rzwnnfxcjp32un9ug7yhhzgtkhvl9l2q74d" \
--contract-gas-limit 2000000 \
--title="$Title" --description="$Description" --deposit="100000000000inj" $TX_OPTS

sleep 5
yes $PASSPHRASE | injectived tx gov vote 4 yes $TX_OPTS