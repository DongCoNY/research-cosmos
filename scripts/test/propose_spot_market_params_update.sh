
#!/bin/bash

PASSPHRASE="12345678"
Title="Spot market params update"
Description="XX"
InstantListingFee="10000inj"
TakerFee="0.01"
MakerFee="0.01"

TX_OPTS="--from=genesis  --chain-id=injective-1 --gas-prices=500000000inj --broadcast-mode=sync --yes"

yes $PASSPHRASE | injectived tx exchange update-spot-market-params $InstantListingFee $TakerFee $MakerFee --title="$Title" --description="$Description" --deposit="100000000000inj" $TX_OPTS
yes $PASSPHRASE | injectived tx gov vote 1 yes $TX_OPTS

