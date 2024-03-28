#!/bin/bash

PASSPHRASE="12345678"
TX_OPTS="--from=genesis --gas-prices=500000000inj  --chain-id=injective-1 --broadcast-mode=sync --yes"

Ticker="ATOM/INJ"
BaseDenom="atom"
QuoteDenom="inj"
MinPriceTickSize="1000000000"
MinQuantityTickSize="0.01"

set -e

echo "Launching $Ticker spot market"
yes $PASSPHRASE | injectived tx exchange instant-spot-market-launch $Ticker $BaseDenom $QuoteDenom --min-price-tick-size="$MinPriceTickSize" --min-quantity-tick-size="$MinQuantityTickSize" $TX_OPTS

sleep 1

echo "Creating buy limit order"
yes $PASSPHRASE | injectived tx exchange create-spot-limit-order "buy" $Ticker $MinQuantityTickSize $MinPriceTickSize $TX_OPTS

sleep 1

echo "Creating sell market order"
yes $PASSPHRASE | injectived tx exchange create-spot-market-order "sell" $Ticker $MinQuantityTickSize $MinPriceTickSize $TX_OPTS
