#!/bin/bash

PASSPHRASE="12345678"
Ticker="ATOM/INJ"
BaseDenom="atom"
QuoteDenom="inj"
MinPriceTickSize="1000000000"
MinQuantityTickSize="0.01"

TX_OPTS="--from=genesis --chain-id=injective-1 --broadcast-mode=sync --gas=2000000 --gas-prices=500000000inj --yes"

yes $PASSPHRASE | injectived tx exchange instant-spot-market-launch $Ticker $BaseDenom $QuoteDenom --min-price-tick-size="$MinPriceTickSize" --min-quantity-tick-size="$MinQuantityTickSize" $TX_OPTS
