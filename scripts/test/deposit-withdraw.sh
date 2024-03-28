#!/bin/bash

PASSPHRASE="12345678"

TX_OPTS="--from=genesis --chain-id=injective-1 --broadcast-mode=sync --gas=2000000 --gas-prices=500000000inj --yes"
yes $PASSPHRASE | injectived tx exchange deposit 1000inj --from=user1 $TX_OPTS

yes $PASSPHRASE | injectived tx exchange withdraw 1000inj --from=user1 $TX_OPTS