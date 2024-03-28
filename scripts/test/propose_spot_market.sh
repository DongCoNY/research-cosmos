#!/bin/bash

PASSPHRASE="12345678"
Title="INJ/ATOM market"
Description="XX"
Ticker="INJ/ATOM"
BaseDenom="inj"
QuoteDenom="atom"
MinPriceTickSize="1000000000"
MinQuantityTickSize="1000000000000000"

TX_OPTS="--from=genesis  --chain-id=injective-1 --gas-prices=500000000inj --broadcast-mode=sync --yes"

yes $PASSPHRASE | injectived tx exchange spot-market-launch "$Ticker" "$BaseDenom" "$QuoteDenom" --min-price-tick-size="$MinPriceTickSize" --min-quantity-tick-size="$MinQuantityTickSize" --title="$Title" --description="$Description" --deposit="100000000000inj" $TX_OPTS
yes $PASSPHRASE | injectived tx gov vote 1 yes $TX_OPTS
