#!/bin/bash

PASSPHRASE="12345678"
Title="Enable Spot Exchange"
Description="Enable Spot Exchange"

TX_OPTS="--chain-id=injective-1 --broadcast-mode=sync --yes "

yes $PASSPHRASE | injectived tx exchange propose-exchange-enable "spot" --title="$Title" --description="$Description" --deposit="100000000000inj" --from=genesis $TX_OPTS
yes $PASSPHRASE | injectived tx gov vote 3 yes --from=genesis $TX_OPTS